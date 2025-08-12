# Project: bc-insights-tui

**A Terminal User Interface for Azure Application Insights**

---

## 1. Overview

bc-insights-tui is a command-line tool for developers to view and query their Azure Application Insights logs directly from the comfort of their terminal. It aims to provide a fast, efficient, and keyboard-driven interface to reduce context switching and streamline the debugging process.

The application will run on Windows, feature a low memory footprint, and provide a rich, interactive experience. A key feature will be an **AI-assisted query builder**, allowing users to generate complex KQL queries using natural language.

---

## 2. User Persona & Use Cases

* **Primary User:** The Business Central Developer.
* **Core Use Cases:**
    * **Live Log Tailing:** Quickly view the latest logs as they come in and scroll through recent history to get an immediate sense of the application's health.
    * **AI-Powered Querying:** Instead of manually writing complex Kusto Query Language (KQL), the developer can describe what they're looking for in plain English (e.g., "show me all failed logins from the last hour"). The tool will use an AI service to translate this into a valid KQL query that can be executed immediately.

---

## 3. Functional Requirements

* **Online-Only:** The application will not support offline mode. All log data will be fetched directly from the Application Insights API on demand.
* **No Local Log Storage:** Logs will not be stored or cached locally on the user's machine.
* **Configurable Fetch Size:** Users can configure the number of logs to retrieve in a single API request.
* **Pagination:** The interface will provide an option to "load more" logs, fetching the next batch of older log entries.
* **Helpful Error Handling:** All authentication and API errors must be user-friendly. Messages should explain the likely cause of the problem and guide the user toward a resolution (e.g., checking permissions in Azure, correcting a config value).
* **Dynamic Table Columns:** When a custom KQL query includes a `project` statement to select specific fields, the log list view must dynamically update its columns to match the fields selected in the query.
* **Custom KQL Queries & Saving:** Users must be able to write their own KQL queries from scratch. The application should provide a mechanism to save these custom queries with a descriptive name for easy reuse.

---

## 4. Core Concepts: Business Central Telemetry

Business Central telemetry in Application Insights is highly dynamic. For our purposes, we treat it as schemaless at the UI layer and render what the service returns without requiring predefined shapes.

* **Primary Data Source:** Most log data is stored in the `traces` table.
* **The `customDimensions` Field:** While standard columns exist, the most valuable, context-rich information for a log entry is stored in the `customDimensions` field, which is a flexible key-value store. Keys and values can vary by event and over time.
* **No hard-coded schemas:** We do not infer or enforce per-event schemas (e.g., by `eventId`). Instead, we display the timestamp directly from the row and show all key-value pairs under `customDimensions` as-is.
* **Dynamic and Extensible:** The set of keys present in `customDimensions` changes across versions and partners. The tool adapts by discovering and rendering these keys dynamically.

Implication: **bc-insights-tui avoids static models**. The details view reads the row timestamp and renders all fields found in `customDimensions`, sorted and formatted for readability, without relying on `eventId`-driven structure.

---

## 5. Technology Stack

This project will be built using Go and the Charm Bracelet ecosystem, chosen for its performance, excellent TUI capabilities, and strong community support.

* **Language:** **Go**
* **TUI Framework:** [**Bubble Tea**](https://github.com/charmbracelet/bubbletea)
* **Styling & Components:** [**Lip Gloss**](https://github.com/charmbracelet/lipgloss) & [**Bubbles**](https://github.com/charmbracelet/bubbles)
* **Authentication:** Standard Go HTTP libraries for OAuth2 Device Authorization Flow.
* **AI Integration:** A client for an AI service like **Azure OpenAI**.

---

## 6. Architecture

The project will follow a modular architecture to separate concerns.

### Directory Structure

```text
bc-insights-tui/
├── main.go                 # Entry point
|
├── tui/                    # All Bubble Tea UI code
│   ├── model.go
│   ├── update.go
│   └── view.go
|
├── auth/                   # Azure OAuth2 logic
│   └── authenticator.go
|
├── appinsights/            # Application Insights API client
│   └── client.go
|
├── ai/                     # AI Service client for KQL generation
│   └── assistant.go
|
└── config/                 # Application configuration
    └── config.go
```

---

## 7. Roadmap

Development will proceed in phases to ensure a solid foundation and iterative progress.

* **Phase 1: Basic TUI Skeleton & Config**
    * [x] Set up the project structure and Go modules.
    * [x] Create a basic Bubble Tea application that can be started and quit.
    * [x] Implement a simple view with a welcome message and help text.
    * [x] Implement basic configuration loading (e.g., for log fetch size).
* **Phase 2: Authentication**
    * [x] Implement the OAuth2 Device Authorization Flow.
    * [x] Create a "Login" view in the TUI.
    * [x] Securely store and refresh authentication tokens.
    * [x] **Ensure all authentication errors are helpful and actionable.**
* **Phase 3: Application Insights Integration**
    * [x] Create the `appinsights` client to make authenticated API calls.
    * [x] Implement a function to execute a basic KQL query that respects the configured fetch size.
    * [x] Create a view to display a list of log entries with default columns.
    * [x] **Implement details view that shows the row timestamp and all fields from `customDimensions` (no per-`eventId` schema).**
    * [ ] Implement pagination logic to "load more" logs.
    * [ ] **Handle API errors and loading states with clear, guiding messages.**
    * [ ] Implement a great visual way to display the result table, that allows horizontal scrolling, dynamics width of columns, and fit inside the general width and height of the ui.
* **Phase 4: Advanced Features**
    * [ ] Implement a detailed view for a single log entry.
    * [ ] **Implement a KQL editor for writing and editing custom queries.**
    * [ ] **Add functionality to save a custom query with a name/description.**
    * [ ] **Implement a query library view to load and run saved queries.**
    * [ ] **Dynamically render table columns based on KQL `project` statements.**
    * [ ] Add color-coding for different log levels.
* **Phase 5: AI-Powered Querying**
    * [ ] Integrate with an AI service (e.g., Azure OpenAI) in the `ai` package.
    * [ ] Create a new TUI view/prompt for natural language input.
    * [ ] Implement logic to send the user's prompt and receive a KQL query.
    * [ ] Allow the user to review, accept, and run the generated query.

---

## 8. Dynamic Column Ranking (Implemented)

The UI ranks discovered `customDimensions` keys to surface the most informative columns first. Ranking combines:
- Presence rate (non-empty frequency)
- Variability (distinct values up to a cap)
- Average value length (penalize overly long textual fields)
- Heuristic type bias (boolean-like and compact categorical fields)
- Regex / keyword boosts (request/operation/error/duration/status/user/session/id patterns + optional AL* prefix rule)

Configuration is exposed via `rank.*` fields in `config.Config` and environment variables (`BCINSIGHTS_RANK_*`). Fail-safe fallback reverts to alphabetical ordering if ranking is disabled or errors occur.

Pinned columns can be forced to the front (`rank.pinned` / `BCINSIGHTS_RANK_PINNED`) maintaining deterministic order for business-critical identifiers.

