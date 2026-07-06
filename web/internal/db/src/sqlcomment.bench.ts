import { bench, describe } from "vitest";
import { annotateSql, staticTagsFromEnv } from "./sqlcomment";

const drizzleSelect =
  "select `keys`.`id`, `keys`.`name` from `keys` where `keys`.`workspace_id` = ? limit ?";

const staticTags = staticTagsFromEnv("dashboard");

describe("sqlcomment benchmarks", () => {
  bench("annotateSql disabled (no service)", () => {
    annotateSql(drizzleSelect, { application: "unkey", service: "" });
  });

  bench("annotateSql drizzle select (static tags only)", () => {
    annotateSql(drizzleSelect, staticTags);
  });

  bench("annotateSql drizzle select with tRPC tags", () => {
    annotateSql(drizzleSelect, staticTags, {
      route: "deploy.envVars.create",
      source: "trpc",
    });
  });
});
