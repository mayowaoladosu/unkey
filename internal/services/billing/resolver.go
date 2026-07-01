// Package billing resolves which rate card is in force for an end-user
// identity and billing period, and prices billable quantities against it.
// It is the single amount-computation path shared by the billing export API
// and the period-close push, so both produce identical money (R19).
package billing

import (
	"context"
	"errors"
	"time"

	"github.com/unkeyed/unkey/pkg/db"
	"github.com/unkeyed/unkey/pkg/fault"
	"github.com/unkeyed/unkey/pkg/ratecard"
	"github.com/unkeyed/unkey/pkg/uid"
)

// ErrNoRateCard means no rate card resolves for the identity: no valid
// end-user selection, no valid assignment, and no workspace default.
var ErrNoRateCard = errors.New("no rate card resolves for this identity")

// ResolvedFrom names the precedence step that produced the card, matching
// the billing_period_rate_cards.resolved_from enum.
type ResolvedFrom string

const (
	ResolvedFromSelection        ResolvedFrom = "selection"
	ResolvedFromAssignment       ResolvedFrom = "assignment"
	ResolvedFromWorkspaceDefault ResolvedFrom = "workspace_default"
)

// ResolvedRateCard is the card in force for one identity and period.
type ResolvedRateCard struct {
	Card         db.RateCard
	Config       ratecard.Config
	ResolvedFrom ResolvedFrom
	// Recorded is true when the resolution came from (or was persisted to)
	// the period record, making it immutable for this period (R18).
	Recorded bool
}

// Resolver implements the KTD7 precedence: recorded period row, else
// end-user selection (if still selectable), else customer assignment, else
// workspace default. Mid-period card changes only affect periods that have
// not been recorded yet.
type Resolver struct {
	database db.Database
}

func NewResolver(database db.Database) *Resolver {
	return &Resolver{database: database}
}

// Resolve returns the card in force for the identity and period without
// persisting anything. Use ResolveAndRecord on billing paths.
func (r *Resolver) Resolve(ctx context.Context, workspaceID, identityID string, year, month int) (ResolvedRateCard, error) {
	recorded, err := db.Query.FindBillingPeriodRateCard(ctx, r.database.RO(), db.FindBillingPeriodRateCardParams{
		WorkspaceID: workspaceID,
		IdentityID:  identityID,
		Year:        int32(year),
		Month:       int32(month),
	})
	if err == nil {
		card, cardErr := r.loadCard(ctx, workspaceID, recorded.RateCardID, false)
		if cardErr != nil {
			return ResolvedRateCard{}, cardErr
		}
		card.ResolvedFrom = ResolvedFrom(recorded.ResolvedFrom)
		card.Recorded = true
		return card, nil
	}
	if !db.IsNotFound(err) {
		return ResolvedRateCard{}, fault.Wrap(err, fault.Internal("failed to look up period rate card record"))
	}

	return r.resolveLive(ctx, workspaceID, identityID)
}

// ResolveAndRecord resolves the card and persists it against the period,
// first write wins: a concurrent recorder or an earlier billing run keeps
// its card, and this call returns whatever ended up recorded (R18).
func (r *Resolver) ResolveAndRecord(ctx context.Context, workspaceID, identityID string, year, month int) (ResolvedRateCard, error) {
	resolved, err := r.Resolve(ctx, workspaceID, identityID, year, month)
	if err != nil {
		return ResolvedRateCard{}, err
	}
	if resolved.Recorded {
		return resolved, nil
	}

	err = db.Query.InsertBillingPeriodRateCard(ctx, r.database.RW(), db.InsertBillingPeriodRateCardParams{
		ID:           uid.New(uid.BillingPeriodRateCardPrefix),
		WorkspaceID:  workspaceID,
		IdentityID:   identityID,
		Year:         int32(year),
		Month:        int32(month),
		RateCardID:   resolved.Card.ID,
		ResolvedFrom: db.BillingPeriodRateCardsResolvedFrom(resolved.ResolvedFrom),
		CreatedAt:    time.Now().UnixMilli(),
	})
	if err != nil {
		return ResolvedRateCard{}, fault.Wrap(err, fault.Internal("failed to record period rate card"))
	}

	// Re-read: INSERT IGNORE means a concurrent writer may have won.
	return r.Resolve(ctx, workspaceID, identityID, year, month)
}

// ResolveLive applies the live precedence (selection -> assignment ->
// workspace default) ignoring any recorded period — the card that will
// govern periods not yet billed.
func (r *Resolver) ResolveLive(ctx context.Context, workspaceID, identityID string) (ResolvedRateCard, error) {
	return r.resolveLive(ctx, workspaceID, identityID)
}

// resolveLive applies selection -> assignment -> workspace default.
func (r *Resolver) resolveLive(ctx context.Context, workspaceID, identityID string) (ResolvedRateCard, error) {
	identity, err := db.Query.FindIdentityByID(ctx, r.database.RO(), db.FindIdentityByIDParams{
		WorkspaceID: workspaceID,
		IdentityID:  identityID,
		Deleted:     false,
	})
	if err != nil {
		if db.IsNotFound(err) {
			return ResolvedRateCard{}, fault.Wrap(err, fault.Internal("identity not found"))
		}
		return ResolvedRateCard{}, fault.Wrap(err, fault.Internal("failed to load identity"))
	}

	// End-user selection wins only while the card is still in the workspace's
	// selectable set; a revoked or archived card falls through silently.
	if identity.SelectedRateCardID.Valid {
		card, cardErr := r.loadCard(ctx, workspaceID, identity.SelectedRateCardID.String, true)
		if cardErr == nil && card.Card.Selectable {
			card.ResolvedFrom = ResolvedFromSelection
			return card, nil
		}
	}

	if identity.RateCardID.Valid {
		card, cardErr := r.loadCard(ctx, workspaceID, identity.RateCardID.String, true)
		if cardErr == nil {
			card.ResolvedFrom = ResolvedFromAssignment
			return card, nil
		}
	}

	settings, err := db.Query.FindWorkspaceBillingSettings(ctx, r.database.RO(), workspaceID)
	if err == nil && settings.DefaultRateCardID.Valid {
		card, cardErr := r.loadCard(ctx, workspaceID, settings.DefaultRateCardID.String, true)
		if cardErr == nil {
			card.ResolvedFrom = ResolvedFromWorkspaceDefault
			return card, nil
		}
	}
	if err != nil && !db.IsNotFound(err) {
		return ResolvedRateCard{}, fault.Wrap(err, fault.Internal("failed to load workspace billing settings"))
	}

	return ResolvedRateCard{}, ErrNoRateCard
}

// loadCard fetches a card and parses its config. When rejectArchived is set,
// archived cards are treated as not found — they stay resolvable only through
// recorded period rows.
func (r *Resolver) loadCard(ctx context.Context, workspaceID, rateCardID string, rejectArchived bool) (ResolvedRateCard, error) {
	card, err := db.Query.FindRateCardByID(ctx, r.database.RO(), db.FindRateCardByIDParams{
		WorkspaceID: workspaceID,
		RateCardID:  rateCardID,
	})
	if err != nil {
		return ResolvedRateCard{}, fault.Wrap(err, fault.Internal("rate card not found"))
	}
	if rejectArchived && card.Archived {
		return ResolvedRateCard{}, fault.New("rate card is archived")
	}
	cfg, err := ratecard.ParseConfig(card.Config)
	if err != nil {
		return ResolvedRateCard{}, fault.Wrap(err, fault.Internal("failed to parse rate card config"))
	}
	return ResolvedRateCard{Card: card, Config: cfg, ResolvedFrom: "", Recorded: false}, nil
}

// Price computes the exact per-dimension and total cents for the quantities
// under the resolved card.
func (c ResolvedRateCard) Price(verifications, credits, ratelimits int64) (ratecard.Amounts, error) {
	return c.Config.Price(verifications, credits, ratelimits)
}
