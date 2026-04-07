# Project Context: The Information Exchange

## Executive Summary

IE is a data marketplace that enables the commodification of information. Formerly impractical because data could not be adequately analyzed before purchase (Arrow's Information Paradox), new trusted AI infrastructure at IE allows buyers to request, find, trust, and use purchaseable information from sellers around the world before making the purchase and without "stealing" the information. This is done through *buyers agents* who enter (or more specifically are cloned into) our walled system and can see and run their analysis with full access to seller data. Their agent can tell them what information to buy, and the exchange brokers the final transaction. If none of the sellers across the market meet the needs of the buyer, they pay nothing, and the agent never returns to reveal the secrets it learned.

The exchange hosts the information for sale, provides the buyer agent platform, and makes money as a broker between the two.

The product is three layers.
The base layer is a raw information market. Sellers post information for sale at a price, buyers agents have tools to search and use this information. Buyers agents can also post requests for information, and sellers can compete to fill those information requests.
The second layer is a trust engine that verifies sellers, cross-references claims, and builds longitudinal accuracy scores from transaction outcomes. This layer is a moat that grows stronger as it learns about the sellers in the market. It provides confidence levels to buyers agents for the information they see.
The third is the buyers agent platform, the AI-powered intelligence layer that hosts buyers agents and allows them to find information to purchase.

This marketplace will form an indispensable backbone of the agent economy, and represents the first of its kind true information marketplace.

> Key insight: prior data marketplaces failed because they were supply-side catalogs of high level descriptions of the information. This was necessary because of Arrow's paradox. However, LLM based AI technology enables a new kind of dense information marketplace using "forgetful buyers agents", AI agents that have the knowledge and goals of their organization but "forget" any information they didn't pay for when they leave the market.

## Workflow Rules

- All plans must become specs checked in to `specs/` before implementation begins.
- Name spec files descriptively (e.g. `mvp-auth-design.md`), not `README.md`.
- Specs live under `specs/features/<feature-name>/` or `specs/<area>/` as appropriate.

## API & CLI Tooling Rules

- **Every new API or service must ship with a CLI tool** (or extend an
  existing CLI tool) that lets a human or Claude exercise the API end
  to end. The CLI is what we will use to populate real demo data, so
  shipping an API without one is incomplete work.
- **CLI tools are written in Go using [spf13/cobra](https://github.com/spf13/cobra).**
  This is the CLI framework we standardize on across the project.

## Branch & PR Rules

- **Never commit directly to master.** All work happens on a feature branch.
- At the start of a session, create or continue work on a descriptive branch (e.g. `add-devops-persona`, `fix-auth-flow`).
- Raise a PR early and update it as you go — commits should be incremental, not one giant squash at the end.
- Every Claude session must end with all changes committed, pushed, and associated with a PR. Do not leave uncommitted work.
- PRs require code review before merging. Do not merge your own PRs without review.
