export type SqlCommentStaticTags = {
  application: string;
  service: string;
  environment?: string;
  region?: string;
  releaseSha?: string;
};

export type SqlCommentDynamicTags = {
  route?: string;
  source?: string;
  operation?: string;
};

const sqlcNameLine = /^--\s*name:\s*(\S+).*?\n/s;

function escapeTagValue(value: string): string {
  return value.replaceAll("\\", "\\\\").replaceAll("'", "\\'");
}

function formatComment(
  staticTags: SqlCommentStaticTags,
  dynamicTags: SqlCommentDynamicTags,
  mode?: string,
  operation?: string,
): string {
  const entries: Array<[string, string | undefined]> = [
    ["application", staticTags.application],
    ["service", staticTags.service],
    ["environment", staticTags.environment],
    ["region", staticTags.region],
    ["release_sha", staticTags.releaseSha],
    ["operation", operation ?? dynamicTags.operation],
    ["route", dynamicTags.route],
    ["source", dynamicTags.source],
    ["mode", mode],
  ];

  const parts = entries
    .filter((entry): entry is [string, string] => Boolean(entry[1]))
    .map(([key, value]) => `${key}='${escapeTagValue(value)}'`);

  if (parts.length === 0) {
    return "";
  }

  return `/*${parts.join(",")}*/`;
}

export function stripSqlcHeader(query: string): { body: string; operation?: string } {
  const match = sqlcNameLine.exec(query);
  if (!match) {
    return { body: query };
  }
  return {
    body: query.slice(match[0].length),
    operation: match[1],
  };
}

export function annotateSql(
  query: string,
  staticTags: SqlCommentStaticTags,
  dynamicTags: SqlCommentDynamicTags = {},
  mode?: string,
): string {
  if (!staticTags.service) {
    return query;
  }

  const { body, operation } = stripSqlcHeader(query);
  const comment = formatComment(staticTags, dynamicTags, mode, operation);
  if (!comment) {
    return body.trim();
  }
  return `${body.trim()} ${comment}`;
}

export function staticTagsFromEnv(service: string): SqlCommentStaticTags {
  const revision = process.env.UNKEY_GIT_COMMIT_SHA ?? process.env.GIT_COMMIT ?? "";
  const releaseSha = revision === "" || revision === "unknown" ? undefined : revision.slice(0, 7);

  return {
    application: "unkey",
    service,
    environment: process.env.UNKEY_ENVIRONMENT,
    region: process.env.UNKEY_REGION ?? process.env.REGION,
    releaseSha,
  };
}
