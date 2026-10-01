import type { UserConfig } from "@commitlint/types";

// The header lint itos.yaml's commits.header_lint delegates to: the header and
// body by config-conventional. The footer rules (commits.footers) are itos's
// own, which it runs beside this in the commit-msg hook, commit check-message
// and verify, so this config carries none.
const config: UserConfig = {
	extends: ["@commitlint/config-conventional"],
};

export default config;
