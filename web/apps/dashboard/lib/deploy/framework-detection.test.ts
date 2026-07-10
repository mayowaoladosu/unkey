import { describe, expect, it } from "vitest";
import { detectFramework } from "./framework-detection";

describe("detectFramework", () => {
  it("returns an explicit unknown result for an empty repository", () => {
    expect(detectFramework({ paths: [], packageJson: null })).toEqual({
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
    });
  });

  it("detects Next.js from an exact package dependency", () => {
    const detection = detectFramework({
      paths: ["package.json", "next.config.mjs", "app/page.tsx"],
      packageJson: {
        dependencies: {
          next: "16.2.6",
          react: "19.2.4",
        },
      },
    });

    expect(detection).toMatchObject({
      preset: { id: "nextjs", name: "Next.js" },
      confidence: "high",
      evidence: [{ kind: "dependency", path: "package.json", value: "next" }],
      runtimeFamily: "node",
      rootCandidates: ["."],
      output: { mode: "mixed", directory: null },
      buildStrategy: "zero-config",
    });
    expect(detection.commandDefaults).toEqual({
      installCommand: null,
      buildCommand: null,
      startCommand: null,
    });
  });

  it("detects Express as a Node server", () => {
    const detection = detectFramework({
      paths: ["package.json", "src/server.ts"],
      packageJson: { dependencies: { express: "5.1.0" } },
    });

    expect(detection).toMatchObject({
      preset: { id: "express", name: "Express" },
      confidence: "high",
      evidence: [{ kind: "dependency", path: "package.json", value: "express" }],
      runtimeFamily: "node",
      rootCandidates: ["."],
      output: { mode: "server", directory: null },
      buildStrategy: "zero-config",
    });
  });

  it("prefers NestJS over its embedded Express dependency", () => {
    const detection = detectFramework({
      paths: ["package.json", "nest-cli.json", "src/main.ts"],
      packageJson: {
        dependencies: {
          "@nestjs/core": "11.1.0",
          express: "5.1.0",
        },
      },
    });

    expect(detection).toMatchObject({
      preset: { id: "nestjs", name: "NestJS" },
      confidence: "high",
      evidence: [{ kind: "dependency", path: "package.json", value: "@nestjs/core" }],
      runtimeFamily: "node",
      output: { mode: "server", directory: null },
    });
  });

  it("detects a Go server from go.mod without guessing a command", () => {
    const detection = detectFramework({ paths: ["go.mod", "cmd/api/main.go"], packageJson: null });

    expect(detection).toMatchObject({
      preset: { id: "go", name: "Go" },
      confidence: "low",
      evidence: [{ kind: "file", path: "go.mod", value: "go.mod" }],
      runtimeFamily: "go",
      rootCandidates: ["."],
      commandDefaults: {
        installCommand: null,
        buildCommand: null,
        startCommand: null,
      },
      output: { mode: "server", directory: null },
      buildStrategy: "zero-config",
    });
  });

  it("requires both canonical Rails markers", () => {
    const complete = detectFramework({
      paths: ["Gemfile", "config/application.rb", "app/controllers/home_controller.rb"],
      packageJson: null,
    });
    const incomplete = detectFramework({
      paths: ["config/application.rb"],
      packageJson: null,
    });

    expect(complete).toMatchObject({
      preset: { id: "rails", name: "Ruby on Rails" },
      confidence: "high",
      evidence: [
        { kind: "file", path: "Gemfile", value: "Gemfile" },
        { kind: "file", path: "config/application.rb", value: "config/application.rb" },
      ],
      runtimeFamily: "ruby",
      rootCandidates: ["."],
      output: { mode: "server", directory: null },
      buildStrategy: "zero-config",
    });
    expect(incomplete.preset?.id).not.toBe("rails");
  });

  it("uses an unambiguous root Dockerfile as the build strategy", () => {
    const detection = detectFramework({
      paths: ["Dockerfile", "src/main.ts"],
      packageJson: null,
    });

    expect(detection).toMatchObject({
      preset: null,
      confidence: "none",
      dockerfileCandidates: ["Dockerfile"],
      recommendedDockerfile: "Dockerfile",
      buildStrategy: "dockerfile",
      output: { mode: "container", directory: null },
    });
  });

  it("leaves multiple nested Dockerfile candidates unresolved", () => {
    const detection = detectFramework({
      paths: ["apps/web/Containerfile.vercel", "apps/api/Dockerfile", "README.md"],
      packageJson: null,
    });

    expect(detection).toMatchObject({
      dockerfileCandidates: ["apps/api/Dockerfile", "apps/web/Containerfile.vercel"],
      recommendedDockerfile: null,
      buildStrategy: "unknown",
      output: { mode: "unknown", directory: null },
      warnings: [
        {
          code: "ambiguous-dockerfiles",
          message: "Multiple container definition files were found.",
        },
      ],
      unresolvedDecisions: [
        {
          code: "select-dockerfile",
          message: "Select the container definition for this service.",
          options: ["apps/api/Dockerfile", "apps/web/Containerfile.vercel"],
        },
      ],
    });
  });

  it("ignores container definitions in dependency and generated directories", () => {
    const detection = detectFramework({
      paths: [
        "node_modules/example/Dockerfile",
        ".next/cache/Containerfile",
        ".git/fixtures/Dockerfile.vercel",
      ],
      packageJson: null,
    });

    expect(detection.dockerfileCandidates).toEqual([]);
    expect(detection.recommendedDockerfile).toBeNull();
    expect(detection.buildStrategy).toBe("unknown");
  });

  it("derives pnpm and its install default from the root lockfile", () => {
    const detection = detectFramework({
      paths: ["package.json", "pnpm-lock.yaml"],
      packageJson: { dependencies: { next: "16.2.6" } },
    });

    expect(detection.packageManager).toBe("pnpm");
    expect(detection.commandDefaults).toEqual({
      installCommand: "pnpm install --frozen-lockfile",
      buildCommand: null,
      startCommand: null,
    });
  });

  it("surfaces conflicting package manager lockfiles", () => {
    const detection = detectFramework({
      paths: ["package.json", "package-lock.json", "pnpm-lock.yaml"],
      packageJson: { dependencies: { next: "16.2.6" } },
    });

    expect(detection.packageManager).toBe("pnpm");
    expect(detection.warnings).toContainEqual({
      code: "conflicting-package-managers",
      message: "Multiple package manager signals were found.",
    });
    expect(detection.unresolvedDecisions).toContainEqual({
      code: "select-package-manager",
      message: "Confirm the package manager for this repository.",
      options: ["pnpm", "npm"],
    });
  });

  it("recommends only commands backed by declared package scripts", () => {
    const detection = detectFramework({
      paths: ["package.json", "pnpm-lock.yaml"],
      packageJson: {
        dependencies: { next: "16.2.6" },
        scripts: {
          build: "next build",
          start: "next start",
        },
      },
    });

    expect(detection.commandDefaults).toEqual({
      installCommand: "pnpm install --frozen-lockfile",
      buildCommand: "pnpm run build",
      startCommand: "pnpm run start",
    });
  });

  it("uses a valid packageManager declaration without requiring a lockfile", () => {
    const detection = detectFramework({
      paths: ["package.json"],
      packageJson: {
        packageManager: "yarn@4.9.2",
        dependencies: { next: "16.2.6" },
        scripts: { build: "next build" },
      },
    });

    expect(detection.packageManager).toBe("yarn");
    expect(detection.commandDefaults).toEqual({
      installCommand: "yarn install",
      buildCommand: "yarn run build",
      startCommand: null,
    });
  });

  it("ignores prototype-inherited manifest fields", () => {
    const inheritedManifest = Object.create({
      dependencies: { next: "16.2.6" },
    }) as unknown;

    const detection = detectFramework({
      paths: ["package.json"],
      packageJson: inheritedManifest,
    });

    expect(detection.preset).toBeNull();
    expect(detection.evidence).toEqual([]);
  });

  it("detects Vite and recommends its conventional static output", () => {
    const detection = detectFramework({
      paths: ["package.json", "pnpm-lock.yaml", "vite.config.ts", "src/main.ts"],
      packageJson: {
        devDependencies: { vite: "7.0.0" },
        scripts: { build: "vite build" },
      },
    });

    expect(detection).toMatchObject({
      preset: { id: "vite", name: "Vite" },
      confidence: "high",
      runtimeFamily: "node",
      rootCandidates: ["."],
      commandDefaults: {
        installCommand: "pnpm install --frozen-lockfile",
        buildCommand: "pnpm run build",
        startCommand: null,
      },
      output: { mode: "static", directory: "dist" },
      buildStrategy: "zero-config",
    });
    expect(detection.evidence).toContainEqual({
      kind: "dependency",
      path: "package.json",
      value: "vite",
    });
  });

  it("detects a plain static repository without inventing a build", () => {
    const detection = detectFramework({
      paths: ["index.html", "styles.css", "assets/logo.svg"],
      packageJson: null,
    });

    expect(detection).toMatchObject({
      preset: { id: "static", name: "Static Site" },
      confidence: "low",
      evidence: [{ kind: "file", path: "index.html", value: "index.html" }],
      packageManager: null,
      runtimeFamily: "static",
      rootCandidates: ["."],
      commandDefaults: {
        installCommand: null,
        buildCommand: null,
        startCommand: null,
      },
      output: { mode: "static", directory: "." },
      buildStrategy: "zero-config",
    });
  });

  it("returns service roots instead of guessing within a monorepo", () => {
    const detection = detectFramework({
      paths: [
        "package.json",
        "pnpm-lock.yaml",
        "pnpm-workspace.yaml",
        "turbo.json",
        "apps/web/package.json",
        "services/api/go.mod",
        "node_modules/ignored/package.json",
        ".next/server/package.json",
      ],
      packageJson: {
        packageManager: "pnpm@11.5.0",
        workspaces: ["apps/*", "services/*"],
      },
    });

    expect(detection.preset).toBeNull();
    expect(detection.rootCandidates).toEqual(["apps/web", "services/api"]);
    expect(detection.monorepo).toEqual({
      detected: true,
      evidence: ["package.json#workspaces", "pnpm-workspace.yaml", "turbo.json"],
    });
    expect(detection.serviceHints).toEqual([
      { root: "apps/web", runtimeFamily: "node" },
      { root: "services/api", runtimeFamily: "go" },
    ]);
    expect(detection.warnings).toContainEqual({
      code: "multiple-service-roots",
      message: "Multiple deployable service roots were found.",
    });
    expect(detection.unresolvedDecisions).toContainEqual({
      code: "select-root-directory",
      message: "Select the root directory for this service.",
      options: ["apps/web", "services/api"],
    });
  });

  it("surfaces disagreement between packageManager and lockfile evidence", () => {
    const detection = detectFramework({
      paths: ["package.json", "package-lock.json"],
      packageJson: {
        packageManager: "yarn@4.9.2",
        dependencies: { next: "16.2.6" },
      },
    });

    expect(detection.packageManager).toBe("yarn");
    expect(detection.commandDefaults.installCommand).toBe("yarn install");
    expect(detection.warnings).toContainEqual({
      code: "conflicting-package-managers",
      message: "Multiple package manager signals were found.",
    });
    expect(detection.unresolvedDecisions).toContainEqual({
      code: "select-package-manager",
      message: "Confirm the package manager for this repository.",
      options: ["yarn", "npm"],
    });
  });

  it("detects Django from its canonical marker over generic Python files", () => {
    const detection = detectFramework({
      paths: ["manage.py", "requirements.txt", "project/wsgi.py"],
      packageJson: null,
    });

    expect(detection).toMatchObject({
      preset: { id: "django", name: "Django" },
      confidence: "medium",
      evidence: [{ kind: "file", path: "manage.py", value: "manage.py" }],
      runtimeFamily: "python",
      rootCandidates: ["."],
      output: { mode: "server", directory: null },
      buildStrategy: "zero-config",
    });
  });

  it("produces service hints independently of Git tree order", () => {
    const paths = ["turbo.json", "apps/api/package.json", "apps/api/go.mod", "apps/web/index.html"];

    const forward = detectFramework({ paths, packageJson: null });
    const reversed = detectFramework({ paths: [...paths].reverse(), packageJson: null });

    expect(forward.serviceHints).toEqual([
      { root: "apps/api", runtimeFamily: "go" },
      { root: "apps/web", runtimeFamily: "static" },
    ]);
    expect(reversed.serviceHints).toEqual(forward.serviceHints);
    expect(reversed.rootCandidates).toEqual(forward.rootCandidates);
  });

  it("requires confirmation for a low-confidence ecosystem match", () => {
    const detection = detectFramework({ paths: ["go.mod"], packageJson: null });

    expect(detection.warnings).toContainEqual({
      code: "low-confidence-framework",
      message: "Framework detection is based on generic ecosystem evidence.",
    });
    expect(detection.unresolvedDecisions).toContainEqual({
      code: "confirm-framework",
      message: "Confirm the detected framework before deploying.",
      options: ["go", "unknown"],
    });
  });

  it("ignores a packageManager declaration with an empty version", () => {
    const detection = detectFramework({
      paths: ["package.json"],
      packageJson: {
        packageManager: "pnpm@",
        dependencies: { next: "16.2.6" },
        scripts: { build: "next build" },
      },
    });

    expect(detection.packageManager).toBeNull();
    expect(detection.commandDefaults).toEqual({
      installCommand: null,
      buildCommand: null,
      startCommand: null,
    });
  });

  it("prefers compound framework evidence over a generic ecosystem marker", () => {
    const detection = detectFramework({
      paths: ["go.mod", "Gemfile", "config/application.rb"],
      packageJson: null,
    });

    expect(detection.preset).toEqual({ id: "rails", name: "Ruby on Rails" });
    expect(detection.confidence).toBe("high");
    expect(detection.evidence).toEqual([
      { kind: "file", path: "Gemfile", value: "Gemfile" },
      { kind: "file", path: "config/application.rb", value: "config/application.rb" },
    ]);
  });

  it("requires review when framework signals have equal precedence", () => {
    const detection = detectFramework({
      paths: ["package.json", "src/server.ts", "src/main.tsx"],
      packageJson: {
        dependencies: { express: "5.1.0" },
        devDependencies: { vite: "7.0.0" },
      },
    });

    expect(detection).toMatchObject({
      preset: null,
      confidence: "none",
      evidence: [
        { kind: "dependency", path: "package.json", value: "vite" },
        { kind: "dependency", path: "package.json", value: "express" },
      ],
      runtimeFamily: "node",
      rootCandidates: ["."],
      output: { mode: "unknown", directory: null },
      buildStrategy: "unknown",
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
    });
  });
});
