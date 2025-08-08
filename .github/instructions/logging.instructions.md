---
applyTo: '**'
description: 'Logging guidelines for debugging and application insights'
---
When an error occurs in the application, it is important to log the error message along with any relevant context to aid in debugging.

It is important that you make extensive use of the logging system to use this as a debugging tool and to gain insights into the application's behavior.
When ever a configuration setting is changed, a log message should be generated to capture the old and new values.
When ever a user action is taken that affects the state of the application, a log message should be generated to capture the details of the action.

When you should fix an issue, make sure to read the most recent log file for insights into what went wrong.