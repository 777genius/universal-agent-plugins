# Build-time bridges

Each `bridges/<id>/bridge.yaml` pins one complete upstream commit and permits
only byte-preserving copies plus complete reviewed files from `overlay/`.
Generated packages are committed at `plugins/<id>/`.

```sh
scripts/build-bridges build <id>
scripts/build-bridges check
```

The builder reads upstream Git blobs as data. It never checks out or executes
upstream content. `check` rebuilds every recipe outside the repository and
compares paths, bytes, and executable modes with the committed package.
