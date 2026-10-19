# Qiniu repository migration

Code, new issues, and releases are maintained in [qiniu/xhub](https://github.com/qiniu/xhub). The initial import preserves all reachable commits and the `main` and `dev` branches from the former repository, based on `main` commit `cc7912c8927b378e25fa7ea5e3fcd7976326218f`. There were no tags or releases at migration time.

## Issue mapping

The manual import preserves issue bodies, checklists, historical comments, and open/closed states. Objects are created by the migration account; attribution, original timestamps, and comment links are included in the imported text. Historical pull requests, reviews, and Actions runs remain in the former repository; their Git commits are preserved.

| Former issue | Qiniu issue | State at migration |
| --- | --- | --- |
| [3](https://github.com/sunqirui1987/xhub/issues/3) | [1](https://github.com/qiniu/xhub/issues/1) | open |
| [6](https://github.com/sunqirui1987/xhub/issues/6) | [2](https://github.com/qiniu/xhub/issues/2) | closed |
| [7](https://github.com/sunqirui1987/xhub/issues/7) | [3](https://github.com/qiniu/xhub/issues/3) | open |
| [8](https://github.com/sunqirui1987/xhub/issues/8) | [4](https://github.com/qiniu/xhub/issues/4) | open |
| [9](https://github.com/sunqirui1987/xhub/issues/9) | [5](https://github.com/qiniu/xhub/issues/5) | closed |
| [15](https://github.com/sunqirui1987/xhub/issues/15) | [6](https://github.com/qiniu/xhub/issues/6) | open |
| [16](https://github.com/sunqirui1987/xhub/issues/16) | [7](https://github.com/qiniu/xhub/issues/7) | open |
| [17](https://github.com/sunqirui1987/xhub/issues/17) | [8](https://github.com/qiniu/xhub/issues/8) | closed |
| [20](https://github.com/sunqirui1987/xhub/issues/20) | [9](https://github.com/qiniu/xhub/issues/9) | closed |

## Publishing and server deployment

- Actions workflows are included in the code import. Image names derive from the repository: `ghcr.io/qiniu/xhub-gateway` and `ghcr.io/qiniu/xhub-console`. Publishing requires unit, backend regression, and browser E2E verification first.
- Pages uses GitHub Actions. The expected site is <https://qiniu.github.io/xhub/>; check the new repository's deployment results for actual availability.
- Example configuration and deployment scripts are tracked. Production configuration, supplier credentials, PostgreSQL, Redis, and user accounts remain on the server and are preserved in place, with private server-side backups. Runtime reports and test credentials are excluded from Git.
- The existing Go module identity `github.com/sunqirui1987/xhub` and internal imports remain compatible. This does not prevent builds or releases from the new repository. A public module rename should be assessed separately for downstream consumers.

## Developer remotes

Keep the former remote as `legacy` if needed and point the primary remote to the new repository:

~~~bash
git remote set-url origin git@github.com:qiniu/xhub.git
git fetch origin
~~~

Check uncommitted changes first. Server updates pin verified image digests rather than following mutable `main` tags.
