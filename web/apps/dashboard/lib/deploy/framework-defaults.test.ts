import { describe, expect, it } from "vitest";
import {
  canAcceptDetectedOutput,
  hasFrameworkDefaults,
  resolveFrameworkDefaults,
} from "./framework-defaults";
import { detectFramework } from "./framework-detection";

describe("resolveFrameworkDefaults", () => {
  it("returns grounded Railpack defaults for a root Vite app", () => {
    const detection = detectFramework({
      paths: ["package.json", "pnpm-lock.yaml", "vite.config.ts"],
      packageJson: {
        devDependencies: { vite: "7.0.0" },
        scripts: { build: "vite build" },
      },
    });

    expect(resolveFrameworkDefaults(detection)).toEqual({
      rootDirectory: ".",
      dockerfile: null,
      buildCommand: "pnpm run build",
    });
  });

  it("makes a nested Dockerfile relative to its unambiguous root", () => {
    const detection = detectFramework({
      paths: ["apps/api/package.json", "apps/api/Dockerfile"],
      packageJson: null,
    });

    expect(resolveFrameworkDefaults(detection)).toEqual({
      rootDirectory: "apps/api",
      dockerfile: "Dockerfile",
      buildCommand: null,
    });
  });

  it("does not turn unresolved repository choices into defaults", () => {
    const detection = detectFramework({
      paths: ["apps/web/package.json", "services/api/go.mod"],
      packageJson: null,
    });
    const defaults = resolveFrameworkDefaults(detection);

    expect(defaults).toEqual({
      rootDirectory: null,
      dockerfile: null,
      buildCommand: null,
    });
    expect(hasFrameworkDefaults(defaults)).toBe(false);
  });

  it("does not apply a build command while its package manager is unresolved", () => {
    const detection = detectFramework({
      paths: ["package.json", "package-lock.json", "pnpm-lock.yaml"],
      packageJson: {
        devDependencies: { vite: "7.0.0" },
        scripts: { build: "vite build" },
      },
    });

    expect(resolveFrameworkDefaults(detection)).toEqual({
      rootDirectory: ".",
      dockerfile: null,
      buildCommand: null,
    });
  });

  it("does not apply a root build command before a monorepo service is selected", () => {
    const detection = detectFramework({
      paths: [
        "package.json",
        "pnpm-lock.yaml",
        "turbo.json",
        "apps/web/package.json",
        "services/api/go.mod",
      ],
      packageJson: {
        dependencies: { next: "16.2.6" },
        scripts: { build: "turbo build" },
      },
    });

    const defaults = resolveFrameworkDefaults(detection);
    expect(defaults).toEqual({
      rootDirectory: null,
      dockerfile: null,
      buildCommand: null,
    });
    expect(hasFrameworkDefaults(defaults)).toBe(false);
  });

  it("allows plain static detection to remain meaningful without setting overrides", () => {
    const detection = detectFramework({
      paths: ["index.html", "styles.css"],
      packageJson: null,
    });
    const defaults = resolveFrameworkDefaults(detection);

    expect(detection.preset?.id).toBe("static");
    expect(detection.output).toEqual({ mode: "static", directory: "." });
    expect(defaults).toEqual({
      rootDirectory: ".",
      dockerfile: null,
      buildCommand: null,
    });
    expect(hasFrameworkDefaults(defaults)).toBe(false);
    expect(canAcceptDetectedOutput(detection, defaults)).toBe(true);
    expect(canAcceptDetectedOutput({ ...detection, preset: null }, defaults)).toBe(false);
    expect(
      canAcceptDetectedOutput(
        {
          ...detection,
          unresolvedDecisions: [
            { code: "confirm-framework", message: "Confirm output", options: ["static"] },
          ],
        },
        defaults,
      ),
    ).toBe(false);
  });
});
