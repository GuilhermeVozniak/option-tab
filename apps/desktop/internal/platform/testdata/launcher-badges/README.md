# Injected launcher badge fixture

The Darwin test compiles this translation unit with production badge extraction and replaces its private read-operation table. AX element objects are created locally for a dummy PID solely to exercise type/owner mapping; no remote AX query, application/window creation, Dock change, event posting, notification access or real badge observation runs.

Assertions cover advertised absence, unreadable and wrong-type values, successful empty and zero strings, oversized-string indicator normalization, cancellation, duplicate matching URLs, exact Dock incarnation change, and target identity invalidation after extraction. Fixture paths are inert strings; canonical-path operations are injected in full-tree tests.

Actual Dock feasibility evidence is recorded separately in the ignored source-feasibility report. This fixture does not prove live badge updates or count meaning on other OS versions.
