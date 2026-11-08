# Third-party notices

This repository's own code is MIT-licensed (see `LICENSE`). It also contains or uses the following third-party software
under its own license:

| Component | Where | License | Source |
| --- | --- | --- | --- |
| axe-core 4.10.2 | `backend/internal/crawler/axe/axe.min.js` (embedded, unmodified) | MPL-2.0 | https://github.com/dequelabs/axe-core |

The MPL-2.0 applies to `axe.min.js` itself, not to the rest of the repository. The file keeps its original license header.
If it is ever modified, the modified file must stay under MPL-2.0 and its source must be made available.

Go and Dart dependencies are fetched by their package managers (`backend/go.mod`, `frontend/pubspec.yaml`) and keep their own licenses.
