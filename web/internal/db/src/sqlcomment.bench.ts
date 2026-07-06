import { bench, describe } from "vitest";
import { annotateSql, staticTagsFromEnv, stripSqlcHeader } from "./sqlcomment";

const sqlcQuery = `-- name: FindKeyForVerification :one
SELECT k.id FROM keys AS k WHERE k.hash = ? AND k.deleted = 0`;

const staticTags = staticTagsFromEnv("dashboard");

describe("sqlcomment benchmarks", () => {
  bench("annotateSql disabled (no service)", () => {
    annotateSql(sqlcQuery, { application: "unkey", service: "" });
  });

  bench("annotateSql sqlc header", () => {
    annotateSql(sqlcQuery, staticTags, {}, "rw");
  });

  bench("annotateSql with route tag", () => {
    annotateSql(sqlcQuery, staticTags, { route: "trpc.keys.create", source: "trpc" }, "rw");
  });

  bench("stripSqlcHeader", () => {
    stripSqlcHeader(sqlcQuery);
  });
});
