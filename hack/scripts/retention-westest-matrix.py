"""Retention acceptance matrix shared by PR, nightly, and release checks."""

import json
import os


def scenarios(profile):
    if profile == "pr":
        return ["local-kind-retention-detach-foreground",
                "local-kind-retention-purge-orphan",
                "local-kind-retention-mixed-background"]
    if profile != "full":
        raise ValueError("Retention profile must be pr or full")
    result = [f"local-kind-retention-{policy}-{propagation}"
              for policy in ("detach", "purge", "mixed")
              for propagation in ("background", "foreground", "orphan")]
    result += [f"local-kind-retention-argocd-{policy}-{operation}-{tracking}"
               for policy in ("detach", "purge")
               for operation in ("prune", "cascade")
               for tracking in ("annotation", "combined")]
    return result + ["local-kind-retention-upgrade"]


if __name__ == "__main__":
    matrix = {"scenario": scenarios(os.environ.get("RETENTION_PROFILE", "pr"))}
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("matrix=" + json.dumps(matrix, separators=(",", ":")) + "\n")
