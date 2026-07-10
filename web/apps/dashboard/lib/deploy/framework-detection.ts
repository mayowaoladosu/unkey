/**
 * Repository metadata available during onboarding without cloning the source.
 * `packageJson` is the parsed root manifest or null when it is absent or invalid.
 */
export type RepositorySnapshot = {
  paths: readonly string[];
  packageJson: unknown;
};

/**
 * Advisory onboarding prediction. Railpack remains authoritative when it
 * creates the build plan from the actual checkout.
 */
export type FrameworkDetection = {
  preset: { id: string; name: string } | null;
  confidence: "none" | "low" | "medium" | "high";
  evidence: Array<{
    kind: "dependency" | "file";
    path: string;
    value: string;
  }>;
  packageManager: "pnpm" | "yarn" | "bun" | "npm" | null;
  runtimeFamily:
    | "node"
    | "python"
    | "go"
    | "rust"
    | "ruby"
    | "php"
    | "jvm"
    | "deno"
    | "static"
    | null;
  rootCandidates: string[];
  commandDefaults: {
    installCommand: string | null;
    buildCommand: string | null;
    startCommand: string | null;
  };
  output: {
    mode: "static" | "server" | "container" | "mixed" | "unknown";
    directory: string | null;
  };
  dockerfileCandidates: string[];
  recommendedDockerfile: string | null;
  buildStrategy: "dockerfile" | "zero-config" | "unknown";
  monorepo: {
    detected: boolean;
    evidence: string[];
  };
  serviceHints: Array<{
    root: string;
    runtimeFamily: NonNullable<FrameworkDetection["runtimeFamily"]>;
  }>;
  warnings: Array<{
    code:
      | "ambiguous-dockerfiles"
      | "ambiguous-frameworks"
      | "conflicting-package-managers"
      | "low-confidence-framework"
      | "multiple-service-roots";
    message: string;
  }>;
  unresolvedDecisions: Array<{
    code:
      | "confirm-framework"
      | "select-dockerfile"
      | "select-framework"
      | "select-package-manager"
      | "select-root-directory";
    message: string;
    options: string[];
  }>;
};

const IGNORED_PATH_SEGMENTS = new Set([".git", ".next", ".nuxt", ".output", "node_modules"]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasOwnString(record: Record<string, unknown>, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(record, key) && typeof record[key] === "string";
}

function getOwnRecord(
  record: Record<string, unknown>,
  key: string,
): Record<string, unknown> | null {
  if (!Object.prototype.hasOwnProperty.call(record, key)) {
    return null;
  }
  const value = record[key];
  return isRecord(value) ? value : null;
}

function declaresDependency(packageJson: unknown, dependency: string): boolean {
  if (!isRecord(packageJson)) {
    return false;
  }

  for (const field of ["dependencies", "devDependencies"]) {
    const dependencies = getOwnRecord(packageJson, field);
    if (dependencies && hasOwnString(dependencies, dependency)) {
      return true;
    }
  }

  return false;
}

function declaresScript(packageJson: unknown, script: string): boolean {
  if (!isRecord(packageJson)) {
    return false;
  }
  const scripts = getOwnRecord(packageJson, "scripts");
  return scripts !== null && hasOwnString(scripts, script);
}

function declaredPackageManager(
  packageJson: unknown,
): NonNullable<FrameworkDetection["packageManager"]> | null {
  if (!isRecord(packageJson) || !hasOwnString(packageJson, "packageManager")) {
    return null;
  }
  const packageManager = packageJson.packageManager;
  if (typeof packageManager !== "string") {
    return null;
  }
  const match = /^(pnpm|yarn|bun|npm)@\S+$/.exec(packageManager);
  if (!match) {
    return null;
  }
  const name = match[1];
  return name === "pnpm" || name === "yarn" || name === "bun" || name === "npm" ? name : null;
}

function normalizePath(path: string): string {
  return path
    .replaceAll("\\", "/")
    .replace(/^(\.\/)+/, "")
    .replace(/^\/+|\/+$/g, "");
}

function isIgnoredPath(path: string): boolean {
  return path.split("/").some((segment) => IGNORED_PATH_SEGMENTS.has(segment.toLowerCase()));
}

function unknownDetection(): FrameworkDetection {
  return {
    preset: null,
    confidence: "none",
    evidence: [],
    packageManager: null,
    runtimeFamily: null,
    rootCandidates: [],
    commandDefaults: {
      installCommand: null,
      buildCommand: null,
      startCommand: null,
    },
    output: {
      mode: "unknown",
      directory: null,
    },
    dockerfileCandidates: [],
    recommendedDockerfile: null,
    buildStrategy: "unknown",
    monorepo: {
      detected: false,
      evidence: [],
    },
    serviceHints: [],
    warnings: [],
    unresolvedDecisions: [],
  };
}

function matchedFramework(match: {
  preset: NonNullable<FrameworkDetection["preset"]>;
  confidence: Exclude<FrameworkDetection["confidence"], "none">;
  evidence: FrameworkDetection["evidence"];
  runtimeFamily: NonNullable<FrameworkDetection["runtimeFamily"]>;
  output: FrameworkDetection["output"];
}): FrameworkDetection {
  return {
    ...unknownDetection(),
    ...match,
    rootCandidates: ["."],
    buildStrategy: "zero-config",
  };
}

function applyDockerfileDetection(
  detection: FrameworkDetection,
  paths: ReadonlySet<string>,
): FrameworkDetection {
  const dockerfileCandidates = [...paths]
    .filter((path) => {
      const basename = path.split("/").at(-1)?.toLowerCase() ?? "";
      return (
        basename === "dockerfile" ||
        basename.startsWith("dockerfile.") ||
        basename === "containerfile" ||
        basename.startsWith("containerfile.")
      );
    })
    .sort((left, right) => {
      const depthDifference = left.split("/").length - right.split("/").length;
      return depthDifference === 0 ? left.localeCompare(right) : depthDifference;
    });

  if (dockerfileCandidates.length === 0) {
    return detection;
  }

  const rootCandidates = dockerfileCandidates.filter((path) => !path.includes("/"));
  const recommendedDockerfile =
    rootCandidates.length === 1
      ? rootCandidates[0]
      : dockerfileCandidates.length === 1
        ? dockerfileCandidates[0]
        : null;

  if (!recommendedDockerfile) {
    return {
      ...detection,
      dockerfileCandidates,
      warnings: [
        ...detection.warnings,
        {
          code: "ambiguous-dockerfiles",
          message: "Multiple container definition files were found.",
        },
      ],
      unresolvedDecisions: [
        ...detection.unresolvedDecisions,
        {
          code: "select-dockerfile",
          message: "Select the container definition for this service.",
          options: dockerfileCandidates,
        },
      ],
    };
  }

  return {
    ...detection,
    dockerfileCandidates,
    recommendedDockerfile,
    buildStrategy: "dockerfile",
    output: { mode: "container", directory: null },
  };
}

function applyPackageManagerDetection(
  detection: FrameworkDetection,
  paths: ReadonlySet<string>,
  packageJson: unknown,
): FrameworkDetection {
  const lockfileManagers: Array<NonNullable<FrameworkDetection["packageManager"]>> = [];
  if (paths.has("pnpm-lock.yaml")) {
    lockfileManagers.push("pnpm");
  }
  if (paths.has("yarn.lock")) {
    lockfileManagers.push("yarn");
  }
  if (paths.has("bun.lock") || paths.has("bun.lockb")) {
    lockfileManagers.push("bun");
  }
  if (paths.has("package-lock.json") || paths.has("npm-shrinkwrap.json")) {
    lockfileManagers.push("npm");
  }

  const manifestPackageManager = declaredPackageManager(packageJson);
  const packageManager = manifestPackageManager ?? lockfileManagers[0];
  if (!packageManager) {
    return detection;
  }
  const managerSignals = [
    ...new Set([...(manifestPackageManager ? [manifestPackageManager] : []), ...lockfileManagers]),
  ];

  const installCommands: Record<NonNullable<FrameworkDetection["packageManager"]>, string> = {
    pnpm: "pnpm install --frozen-lockfile",
    yarn: "yarn install --frozen-lockfile",
    bun: "bun install --frozen-lockfile",
    npm: "npm ci",
  };
  const manifestInstallCommands: Record<
    NonNullable<FrameworkDetection["packageManager"]>,
    string
  > = {
    pnpm: "pnpm install",
    yarn: "yarn install",
    bun: "bun install",
    npm: "npm install",
  };
  const result: FrameworkDetection = {
    ...detection,
    packageManager,
    commandDefaults: {
      installCommand: lockfileManagers.includes(packageManager)
        ? installCommands[packageManager]
        : manifestInstallCommands[packageManager],
      buildCommand: declaresScript(packageJson, "build") ? `${packageManager} run build` : null,
      startCommand: declaresScript(packageJson, "start") ? `${packageManager} run start` : null,
    },
  };

  if (managerSignals.length <= 1) {
    return result;
  }

  return {
    ...result,
    warnings: [
      ...result.warnings,
      {
        code: "conflicting-package-managers",
        message: "Multiple package manager signals were found.",
      },
    ],
    unresolvedDecisions: [
      ...result.unresolvedDecisions,
      {
        code: "select-package-manager",
        message: "Confirm the package manager for this repository.",
        options: managerSignals,
      },
    ],
  };
}

function sortPaths(paths: string[]): string[] {
  return paths.sort((left, right) => {
    const depthDifference = left.split("/").length - right.split("/").length;
    return depthDifference === 0 ? left.localeCompare(right) : depthDifference;
  });
}

function hasWorkspaceDeclaration(packageJson: unknown): boolean {
  if (!isRecord(packageJson) || !Object.prototype.hasOwnProperty.call(packageJson, "workspaces")) {
    return false;
  }
  return Array.isArray(packageJson.workspaces) || isRecord(packageJson.workspaces);
}

function findServiceHints(paths: ReadonlySet<string>): FrameworkDetection["serviceHints"] {
  const markers: Array<{
    suffix: string;
    runtimeFamily: NonNullable<FrameworkDetection["runtimeFamily"]>;
  }> = [
    { suffix: "config/application.rb", runtimeFamily: "ruby" },
    { suffix: "manage.py", runtimeFamily: "python" },
    { suffix: "go.mod", runtimeFamily: "go" },
    { suffix: "Cargo.toml", runtimeFamily: "rust" },
    { suffix: "deno.json", runtimeFamily: "deno" },
    { suffix: "deno.jsonc", runtimeFamily: "deno" },
    { suffix: "composer.json", runtimeFamily: "php" },
    { suffix: "pom.xml", runtimeFamily: "jvm" },
    { suffix: "build.gradle", runtimeFamily: "jvm" },
    { suffix: "build.gradle.kts", runtimeFamily: "jvm" },
    { suffix: "requirements.txt", runtimeFamily: "python" },
    { suffix: "pyproject.toml", runtimeFamily: "python" },
    { suffix: "Gemfile", runtimeFamily: "ruby" },
    { suffix: "package.json", runtimeFamily: "node" },
    { suffix: "index.html", runtimeFamily: "static" },
  ];
  const hints = new Map<string, FrameworkDetection["serviceHints"][number]>();
  const sortedRepositoryPaths = [...paths].sort((left, right) => left.localeCompare(right));

  for (const marker of markers) {
    for (const path of sortedRepositoryPaths) {
      const markerPath = `/${marker.suffix}`;
      if (!path.endsWith(markerPath)) {
        continue;
      }
      const root = path.slice(0, -markerPath.length);
      if (root && !hints.has(root)) {
        hints.set(root, { root, runtimeFamily: marker.runtimeFamily });
      }
    }
  }

  return sortPaths([...hints.keys()])
    .map((root) => hints.get(root))
    .filter((hint) => hint !== undefined);
}

function applyRepositoryShapeDetection(
  detection: FrameworkDetection,
  paths: ReadonlySet<string>,
  packageJson: unknown,
): FrameworkDetection {
  const serviceHints = findServiceHints(paths);
  const rootCandidates = sortPaths([
    ...new Set([...detection.rootCandidates, ...serviceHints.map((hint) => hint.root)]),
  ]);
  const monorepoEvidence: string[] = [];
  if (hasWorkspaceDeclaration(packageJson)) {
    monorepoEvidence.push("package.json#workspaces");
  }
  for (const marker of [
    "pnpm-workspace.yaml",
    "turbo.json",
    "nx.json",
    "lerna.json",
    "rush.json",
  ]) {
    if (paths.has(marker)) {
      monorepoEvidence.push(marker);
    }
  }

  const result: FrameworkDetection = {
    ...detection,
    rootCandidates,
    monorepo: {
      detected: monorepoEvidence.length > 0 || rootCandidates.length > 1,
      evidence: monorepoEvidence,
    },
    serviceHints,
  };

  if (rootCandidates.length <= 1) {
    return result;
  }

  return {
    ...result,
    warnings: [
      ...result.warnings,
      {
        code: "multiple-service-roots",
        message: "Multiple deployable service roots were found.",
      },
    ],
    unresolvedDecisions: [
      ...result.unresolvedDecisions,
      {
        code: "select-root-directory",
        message: "Select the root directory for this service.",
        options: rootCandidates,
      },
    ],
  };
}

function applyConfidenceReview(detection: FrameworkDetection): FrameworkDetection {
  if (
    detection.confidence !== "low" ||
    detection.runtimeFamily === "static" ||
    detection.preset === null
  ) {
    return detection;
  }

  return {
    ...detection,
    warnings: [
      ...detection.warnings,
      {
        code: "low-confidence-framework",
        message: "Framework detection is based on generic ecosystem evidence.",
      },
    ],
    unresolvedDecisions: [
      ...detection.unresolvedDecisions,
      {
        code: "confirm-framework",
        message: "Confirm the detected framework before deploying.",
        options: [detection.preset.id, "unknown"],
      },
    ],
  };
}

function finalizeDetection(
  detection: FrameworkDetection,
  paths: ReadonlySet<string>,
  packageJson: unknown,
): FrameworkDetection {
  return applyRepositoryShapeDetection(
    applyDockerfileDetection(
      applyPackageManagerDetection(applyConfidenceReview(detection), paths, packageJson),
      paths,
    ),
    paths,
    packageJson,
  );
}

export function detectFramework(snapshot: RepositorySnapshot): FrameworkDetection {
  const detection = unknownDetection();
  const paths = new Set(
    snapshot.paths.map(normalizePath).filter((path) => path.length > 0 && !isIgnoredPath(path)),
  );
  const hasNext = declaresDependency(snapshot.packageJson, "next");
  const hasNest = declaresDependency(snapshot.packageJson, "@nestjs/core");
  const hasVite = declaresDependency(snapshot.packageJson, "vite");
  const hasExpress = declaresDependency(snapshot.packageJson, "express");

  if (hasVite && hasExpress && !hasNext && !hasNest) {
    return finalizeDetection(
      {
        ...detection,
        evidence: [
          { kind: "dependency", path: "package.json", value: "vite" },
          { kind: "dependency", path: "package.json", value: "express" },
        ],
        runtimeFamily: "node",
        rootCandidates: ["."],
        warnings: [
          {
            code: "ambiguous-frameworks",
            message: "Multiple framework signals have equal precedence.",
          },
        ],
        unresolvedDecisions: [
          {
            code: "select-framework",
            message: "Select the framework preset for this service.",
            options: ["vite", "express"],
          },
        ],
      },
      paths,
      snapshot.packageJson,
    );
  }

  if (hasNext) {
    return finalizeDetection(
      matchedFramework({
        preset: { id: "nextjs", name: "Next.js" },
        confidence: "high",
        evidence: [{ kind: "dependency", path: "package.json", value: "next" }],
        runtimeFamily: "node",
        output: { mode: "mixed", directory: null },
      }),
      paths,
      snapshot.packageJson,
    );
  }

  if (hasNest) {
    return finalizeDetection(
      matchedFramework({
        preset: { id: "nestjs", name: "NestJS" },
        confidence: "high",
        evidence: [{ kind: "dependency", path: "package.json", value: "@nestjs/core" }],
        runtimeFamily: "node",
        output: { mode: "server", directory: null },
      }),
      paths,
      snapshot.packageJson,
    );
  }

  if (hasVite) {
    return finalizeDetection(
      matchedFramework({
        preset: { id: "vite", name: "Vite" },
        confidence: "high",
        evidence: [{ kind: "dependency", path: "package.json", value: "vite" }],
        runtimeFamily: "node",
        output: { mode: "static", directory: "dist" },
      }),
      paths,
      snapshot.packageJson,
    );
  }

  if (hasExpress) {
    return finalizeDetection(
      matchedFramework({
        preset: { id: "express", name: "Express" },
        confidence: "high",
        evidence: [{ kind: "dependency", path: "package.json", value: "express" }],
        runtimeFamily: "node",
        output: { mode: "server", directory: null },
      }),
      paths,
      snapshot.packageJson,
    );
  }

  if (paths.has("Gemfile") && paths.has("config/application.rb")) {
    return finalizeDetection(
      matchedFramework({
        preset: { id: "rails", name: "Ruby on Rails" },
        confidence: "high",
        evidence: [
          { kind: "file", path: "Gemfile", value: "Gemfile" },
          { kind: "file", path: "config/application.rb", value: "config/application.rb" },
        ],
        runtimeFamily: "ruby",
        output: { mode: "server", directory: null },
      }),
      paths,
      snapshot.packageJson,
    );
  }

  if (paths.has("manage.py")) {
    return finalizeDetection(
      matchedFramework({
        preset: { id: "django", name: "Django" },
        confidence: "medium",
        evidence: [{ kind: "file", path: "manage.py", value: "manage.py" }],
        runtimeFamily: "python",
        output: { mode: "server", directory: null },
      }),
      paths,
      snapshot.packageJson,
    );
  }

  if (paths.has("go.mod")) {
    return finalizeDetection(
      matchedFramework({
        preset: { id: "go", name: "Go" },
        confidence: "low",
        evidence: [{ kind: "file", path: "go.mod", value: "go.mod" }],
        runtimeFamily: "go",
        output: { mode: "server", directory: null },
      }),
      paths,
      snapshot.packageJson,
    );
  }

  if (paths.has("index.html")) {
    return finalizeDetection(
      matchedFramework({
        preset: { id: "static", name: "Static Site" },
        confidence: "low",
        evidence: [{ kind: "file", path: "index.html", value: "index.html" }],
        runtimeFamily: "static",
        output: { mode: "static", directory: "." },
      }),
      paths,
      snapshot.packageJson,
    );
  }

  return finalizeDetection(detection, paths, snapshot.packageJson);
}
