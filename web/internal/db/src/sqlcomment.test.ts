import { describe, expect, it } from "vitest";
import { annotateSql, staticTagsFromEnv, stripSqlcHeader } from "./sqlcomment";

describe("sqlcomment", () => {
  it("extracts sqlc operation names", () => {
    const query = `-- name: FindKeyForVerification :one
select id from keys`;

    expect(stripSqlcHeader(query)).toEqual({
      body: "select id from keys",
      operation: "FindKeyForVerification",
    });
  });

  it("appends SQLCommenter tags", () => {
    const got = annotateSql(
      "select 1",
      {
        application: "unkey",
        service: "dashboard",
        region: "us-east-1",
      },
      { route: "trpc.keys.create", source: "app" },
      "rw",
    );

    expect(got).toContain("service='dashboard'");
    expect(got).toContain("route='trpc.keys.create'");
    expect(got).toContain("mode='rw'");
  });

  it("builds static tags from env", () => {
    const previous = process.env.UNKEY_GIT_COMMIT_SHA;
    process.env.UNKEY_GIT_COMMIT_SHA = "abcdef012345";
    try {
      expect(staticTagsFromEnv("portal").releaseSha).toBe("abcdef0");
    } finally {
      process.env.UNKEY_GIT_COMMIT_SHA = previous;
    }
  });
});
