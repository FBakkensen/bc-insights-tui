TUI Mockup Design: The Command Palette Driven ApproachThis document outlines the user interface design for bc-insights-tui, which is centered around a powerful, keyboard-driven command palette. This approach keeps the main interface clean and focused on displaying logs, while all actions—from querying to configuration—are handled through a consistent command input.1. Login FlowWhen the user first starts the application without valid credentials, they are presented with the device authentication flow. The UI provides clear instructions on how to log in using a web browser.Interaction Model:The application displays a user code and a verification URL.The user opens the URL in their browser and enters the code to authorize the application.The TUI will show a spinner and automatically proceed once authentication is complete.
+--------------------------------------------------------------------------------------------------+
| bc-insights-tui | Authentication Required                                                        |
+--------------------------------------------------------------------------------------------------+
|                                                                                                  |
|   Please sign in to your Azure account:                                                          |
|                                                                                                  |
|   1. Open a browser and go to:                                                                   |
|      https://microsoft.com/devicelogin                                                           |
|                                                                                                  |
|   2. Enter the following code:                                                                   |
|      A1B2C3D4E                                                                                   |
|                                                                                                  |
|   [•] Waiting for authentication...                                                              |
|                                                                                                  |
|                                                                                                  |
|                                                                                                  |
|                                                                                                  |
+--------------------------------------------------------------------------------------------------+
| [Ctrl+C] Cancel Login                                                                            |
+--------------------------------------------------------------------------------------------------+
2. Main View (Default State)Once logged in, the user sees a clean, readable table of the most recent logs. The status bar provides context and hints at the primary action.Interaction Model:Use arrow keys (↑/↓) to navigate the log list.Press Enter on a log to view its details.Press Ctrl+P to open the command palette.
+---------------------------------------------------------------------------------------------------+
| bc-insights-tui | Press Ctrl+P to open command palette                                            |
+---------------------------------------------------------------------------------------------------+
| Timestamp           | Level  | Event ID | Message                                                 |
|---------------------|------- |----------|---------------------------------------------------------|
| 2025-08-03 10:15:10 | INFO   | AL0000E26| Job queue entry {Job Queue Id} finished.                |
| 2025-08-03 10:15:08 | VERBOSE| RT0005   | Long running SQL query...                               |
| 2025-08-03 10:15:02 | INFO   | RT0008   | Incoming web service call...                            |
| 2025-08-03 10:14:55 | WARN   | RT0019   | Outgoing web service call failed: 404                   |
| 2025-08-03 10:14:51 | INFO   | AL0000E25| Job queue entry {Job Queue Id} started.                 |
| ...                                                                                               |
+---------------------------------------------------------------------------------------------------+
| [Ctrl+P] Command Palette | [↑↓] Navigate | [Enter] Details | [Ctrl+C] Quit                        |
+---------------------------------------------------------------------------------------------------+
3. AI Query InteractionThe user opens the palette (Ctrl+P) and types a natural language query prefixed with ai:. The log view updates instantly upon execution.Interaction Model:Type ai: followed by a plain English request.Press Enter to have the AI generate and run the corresponding KQL query.+--------------------------------------------------------------------------------------------------+
| > ai: show all job queue errors from today                                                       |
+--------------------------------------------------------------------------------------------------+
| Timestamp           | Level | Event ID | Message                                                 |
|---------------------|-------|----------|---------------------------------------------------------|
| 2025-08-03 08:15:22 | ERROR | AL0000HE7| Job queue entry {Job Queue Id} errored.                 |
| 2025-08-03 07:40:11 | ERROR | AL0000HE7| Job queue entry {Job Queue Id} errored.                 |
| ...                                                                                              |
+--------------------------------------------------------------------------------------------------+
| [Enter] Run Query | [Esc] Cancel | [Ctrl+C] Quit                                                 |
+--------------------------------------------------------------------------------------------------+
4. Simple Filter InteractionFor quick filtering without using AI, the user can use the filter: command to narrow down logs based on text in any column.Interaction Model:Type filter: followed by the text to search for (e.g., an eventId).The log list updates as you type, showing only matching entries.
+---------------------------------------------------------------------------------------------------+
| > filter: RT0005                                                                                  |
+---------------------------------------------------------------------------------------------------+
| Timestamp           | Level  | Event ID | Message                                                 |
|---------------------|------- |----------|---------------------------------------------------------|
| 2025-08-03 10:15:08 | VERBOSE| RT0005   | Long running SQL query...                               |
| 2025-08-03 10:12:33 | VERBOSE| RT0005   | Long running SQL query...                               |
| 2025-08-03 10:09:01 | VERBOSE| RT0005   | Long running SQL query...                               |
| ...                                                                                               |
+---------------------------------------------------------------------------------------------------+
| [Enter] Apply Filter | [Esc] Cancel | [Ctrl+C] Quit                                               |
+---------------------------------------------------------------------------------------------------+
5. Viewing Log DetailsPressing Enter on any log opens a modal window with its complete details, including all customDimensions. This keeps the background context visible while focusing on the selected log.Interaction Model:Press Esc to close the modal and return to the log list.
+--------------------------------------------------------------------------------------------------+
| bc-insights-tui | Press Ctrl+P to open command palette                                           |
+--------------------------------------------------------------------------------------------------+
| 2025-08-03 10:15:10 | INFO   | AL0000E26| Job q+-----------------------------------------------+ |
| 2025-08-03 10:15:08 | VERBOSE| RT0005   | Lon… | Log Details (RT0019)     [Press ESC to close] | |
| 2025-08-03 10:15:02 | INFO   | RT0008   | Inc… |-----------------------------------------------| |
| 2025-08-03 10:14:55 | WARN   | RT0019   | Out… | timestamp: 2025-08-03 10:14:55                | |
| 2025-08-03 10:14:51 | INFO   | AL0000E25| Job q| severityLevel: 2                              | |
| ...                 |        |          |      | customDimensions:                             | |
|                     |        |          |      |   alHttpStatus: "404"                         | |
|                     |        |          |      |   alUrl: "https://api.example.com/data"       | |
|                     |        |          |      |   ...                                         | |
|                     |        |          |      +-----------------------------------------------+ |
|                     |        |          |                                                        |
+--------------------------------------------------------------------------------------------------+
| [Ctrl+P] Command Palette | [↑↓] Navigate | [Enter] Details | [Ctrl+C] Quit                       |
+--------------------------------------------------------------------------------------------------+
6. Changing SettingsUsers can view and modify configuration settings directly from the command palette using the set: command.Interaction Model:Type set to see a list of available settings.Type set <setting>=<value> (e.g., set fetchSize=200) to change a setting.The application confirms the change and uses the new setting for subsequent actions.
+--------------------------------------------------------------------------------------------------+
| > set fetchSize=200                                                                              |
+--------------------------------------------------------------------------------------------------+
|                                                                                                  |
|   > set fetchSize=200                                                                            |
|     ✓ fetchSize updated to 200                                                                   |
|                                                                                                  |
|   > set                                                                                          |
|     - fetchSize: 100                                                                             |
|     - environment: MyProdEnvironment                                                             |
|                                                                                                  |
|                                                                                                  |
|                                                                                                  |
|                                                                                                  |
|                                                                                                  |
|                                                                                                  |
+--------------------------------------------------------------------------------------------------+
| [Enter] Apply | [Esc] Close Palette | [Ctrl+C] Quit                                              |
+--------------------------------------------------------------------------------------------------+
7. Dynamic Columns from Custom QueriesWhen a KQL query includes a project statement to select specific fields, the main log view will adapt its columns to match the query's output. This is essential for both AI-generated and user-written custom queries.Interaction Model:The application parses the KQL query for a project statement.If found, the table headers in the log list are dynamically replaced with the projected field names.The table cells are populated with the corresponding data from the query result.+-------------------------------------------------------------------------------------------+
| > ai: show me the user and method for all long running sql queries                               |
+--------------------------------------------------------------------------------------------------+
| timestamp           | alUser                               | alMethod                            |
|---------------------|--------------------------------------|-------------------------------------|
| 2025-08-03 10:15:08 | USER1@domain.com                     | ProcessSalesOrder(Page 42)          |
| 2025-08-03 10:12:33 | USER2@domain.com                     | PostInvoice(Codeunit 80)            |
| 2025-08-03 10:09:01 | USER1@domain.com                     | CalculateInventory(Report 1001)     |
| ...                                                                                              |
+--------------------------------------------------------------------------------------------------+
| [Enter] Run Query | [Esc] Cancel | [Ctrl+C] Quit                                                 |
+--------------------------------------------------------------------------------------------------+
