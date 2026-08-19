# IDATA

A local Vue and Element Plus demo application.

## Run

Python 3 is the only local runtime requirement:

```shell
python start.py
```

Open `http://localhost:54321` in Google Chrome. If another process is listening
on port `54321`, the launcher exits with a clear error. Stop the existing
process and run the launcher again.

The inspection model defaults to `Qwen3.8-27B-Q4`. Supply its API key through
the `IDATA_MODEL_API_KEY` environment variable so credentials never enter
Git history.

To stop the local application from another terminal:

```shell
python stop.py
```

The page loads the approved, pinned Vue `3.5.24` and Element Plus `2.11.8`
browser builds from the sibling `vendor` directory. No internet connection is
required at runtime.
