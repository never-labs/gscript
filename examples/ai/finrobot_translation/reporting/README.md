# FinRobot Reporting/Web Translation Skeleton

This directory records the FinRobot reporting boundary as ordinary Leia project
composition, not as a finance-specific dialect.

The skeleton in `../reporting.leia` maps these FinRobot source areas:

- `finrobot/functional/charting.py`: stock price, share-performance, and PE/EPS
  chart helpers become chart specs over source tables.
- `finrobot/functional/reportlab.py`: annual-report sections and chart image
  paths become report-section data plus a PDF-export boundary.
- `finrobot_equity/core/src/modules/report_structure.py`: section order,
  required-section validation, source annotations, and AI disclosure become
  ordinary report metadata.
- `finrobot_equity/core/src/modules/chart_generator.py` and
  `enhanced_chart_generator.py`: concrete matplotlib generation becomes
  package-owned chart renderers referenced by spec names.
- `finrobot_equity/core/src/modules/html_renderer.py`: combined HTML rendering
  becomes an HTML artifact boundary fed by validated sections and chart outputs.
- `finrobot_equity/core/src/modules/pdf_generator.py` and
  `professional_pdf_report.py`: styled PDF generation becomes a planned
  document-export step.
- `finrobot_equity/web_app/*`: FastAPI routes, auth, logs, task state, output
  directories, and history become web orchestration around the same analysis,
  report, and PDF stages.

The example deliberately does not start a web server, call provider APIs, render
matplotlib images, or write PDF bytes. Those are package/runtime integration
gaps, tracked in `../GAPS.md`.
