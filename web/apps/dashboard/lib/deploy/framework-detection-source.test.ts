import { describe, expect, it } from "vitest";
import { detectionMatchesGitSource } from "./framework-detection-source";

describe("detectionMatchesGitSource", () => {
  it("accepts detection for the current repository and branch", () => {
    expect(
      detectionMatchesGitSource(
        { repositoryFullName: "acme/app", branch: "main" },
        { repositoryFullName: "acme/app", branch: "main" },
      ),
    ).toBe(true);
  });

  it("rejects detection after the repository or branch changes", () => {
    expect(
      detectionMatchesGitSource(
        { repositoryFullName: "acme/app", branch: "main" },
        { repositoryFullName: "acme/other", branch: "main" },
      ),
    ).toBe(false);
    expect(
      detectionMatchesGitSource(
        { repositoryFullName: "acme/app", branch: "main" },
        { repositoryFullName: "acme/app", branch: "preview" },
      ),
    ).toBe(false);
  });
});
