# Instructions for AI Agents
These rules apply to this directory and all descendants containing the project's main application code.
The project will use Vue as its frontend framework; this framework choice is approved.
The project will use Element Plus as its Vue UI component library; this dependency is approved.
Use stable release versions for all tools and dependencies, including Vue and Element Plus; do not use alpha, beta, release-candidate, nightly, canary, or other prerelease versions.
When building or modifying the project, use clear, professional, and formal English for all names, labels, messages, documentation, and other user-facing or developer-facing text. Treat wording supplied by the owner as an expression of intent rather than necessarily final copy, and appropriately rename or rephrase it while preserving its intended meaning, because the owner is not a native English speaker.
Obtain explicit prior approval from the project's human owner before making any other architectural or project-structure choice, or before adding any new dependency.
Do not introduce additional frameworks, libraries, packages, build tools, or Vue ecosystem dependencies unless the project's human owner has approved them first.
For each new feature, first determine whether Vue or Element Plus already provides a suitable capability, and use it when available.
If neither Vue nor Element Plus provides a suitable solution, research mature, widely used Vue components or component packages before proposing a custom implementation.
Present the researched package options and the custom implementation option to the human owner, including their relevant tradeoffs. Obtain explicit approval for the selected approach before adding a dependency or writing the custom implementation.
The application must run on both macOS and Windows.
Use Google Chrome as the primary supported browser.
The application must use only port `54321`.
The canonical local URL is `http://localhost:54321`.
Never use fallback, random, auto-incremented, or alternative ports.
Use one shared Python starting file on macOS and Windows; do not create or rely on `.cmd` or `.bat` launchers.
Fail clearly if port `54321` is unavailable, and never hard-code machine-specific absolute paths.
Do not ask for approval before running Chrome-related commands needed to test the application.
Before completion, run available checks and verify the app in Chrome at `http://localhost:54321`.
After completing work, leave the application running at `http://localhost:54321` so the human owner can test it manually.
