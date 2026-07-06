import { bench, describe } from "vitest";
import { annotateSql, staticTagsFromEnv } from "./sqlcomment";

const drizzleSelect =
  "select `keys`.`id`, `keys`.`name` from `keys` where `keys`.`workspace_id` = ? limit ?";

const staticTags = staticTagsFromEnv("dashboard");

describe("sqlcomment benchmarks", () => {
  bench("tagging disabled (empty service)", () => {
    annotateSql(drizzleSelect, { application: "unkey", service: "" });
  });

  bench("tagging enabled (static tags only)", () => {
    annotateSql(drizzleSelect, staticTags);
  });

  bench("tagging enabled (static + tRPC route/source)", () => {
    annotateSql(drizzleSelect, staticTags, {
      route: "deploy.envVars.create",
      source: "trpc",
    });
  });
});
