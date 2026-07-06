import { AsyncLocalStorage } from "node:async_hooks";
import type { Pool, PoolOptions } from "mysql2/promise";
import mysql from "mysql2/promise";
import { type SqlCommentDynamicTags, type SqlCommentStaticTags, annotateSql } from "./sqlcomment";

type Queryable = Pick<Pool, "query" | "execute">;

function wrapQueryable<T extends Queryable>(pool: T, staticTags: SqlCommentStaticTags): T {
  const annotateArg = (sql: unknown): unknown => {
    if (typeof sql !== "string") {
      return sql;
    }
    return annotateSql(sql, staticTags, dynamicTagsFromStore());
  };

  return new Proxy(pool, {
    get(target, property, receiver) {
      if (property === "query" || property === "execute") {
        return (sql: unknown, ...args: unknown[]) => {
          const annotated = annotateArg(sql);
          return Reflect.apply(target[property as "query" | "execute"], target, [
            annotated,
            ...args,
          ]);
        };
      }
      return Reflect.get(target, property, receiver);
    },
  });
}

const dynamicStore = new AsyncLocalStorage<SqlCommentDynamicTags>();

export function runWithSqlCommentTags<T>(tags: SqlCommentDynamicTags, fn: () => T): T {
  return dynamicStore.run(tags, fn);
}

export function dynamicTagsFromStore(): SqlCommentDynamicTags {
  return dynamicStore.getStore() ?? {};
}

export function createCommentedPool(options: PoolOptions, staticTags: SqlCommentStaticTags): Pool {
  const pool = mysql.createPool(options);
  return wrapQueryable(pool, staticTags);
}
