# pytorch

PyTorch with JupyterLab, from the
[Jupyter Docker Stacks](https://jupyter-docker-stacks.readthedocs.io/) images.
Runs on CPU anywhere, or on an NVIDIA GPU when you have one.

```
spin up pytorch             # CPU
spin up pytorch --gpu       # NVIDIA GPU
spin open pytorch           # open JupyterLab, token already in the URL
```

## Ports

| Service           | Host port | Container | Env var             |
|-------------------|-----------|-----------|---------------------|
| jupyter(-gpu)     | `8888`    | 8888      | `JUPYTER_PORT`      |
| TensorBoard       | `6006`    | 6006      | `TENSORBOARD_PORT`  |

Nothing listens on 6006 until you start TensorBoard yourself, from a notebook
terminal:

```
tensorboard --logdir ./work/runs --host 0.0.0.0
```

## Access

JupyterLab is token-authenticated. The token is `spinup` by default:

```
http://localhost:8888/lab?token=spinup
```

Treat it as a password — anyone with it can execute arbitrary code in the
container. Change it with `spin env pytorch --edit`.

## CPU and GPU are separate services

This stack is unusual: it defines **two** services on the same ports, so both
sit behind a Compose profile and neither starts on its own.

| Profile | Service       | Image                                             |
|---------|---------------|---------------------------------------------------|
| `cpu`   | `jupyter`     | `pytorch-notebook:pytorch-2.13.0`                 |
| `gpu`   | `jupyter-gpu` | `pytorch-notebook:cuda12-pytorch-2.11.0`          |

`spinup` selects `cpu` automatically (`default_profiles` in `spinup.yaml`) and
swaps to `gpu` for `--gpu`. Using plain Compose, pick one explicitly:

```
docker compose --profile cpu up -d
docker compose --profile gpu up -d
```

Running both at once fails on a port collision — they are alternatives, not
companions.

The GPU image lags the CPU one (PyTorch 2.11 vs 2.13) because that is the newest
CUDA build upstream publishes. If you need matching versions, pin both to the
`cuda12-*` line.

### GPU requirements

`--gpu` needs an NVIDIA GPU, the proprietary driver, and the
[NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)
on the host. Without them Docker reports `could not select device driver "nvidia"`.
`spin doctor` checks for this.

There is no GPU path on macOS — Docker Desktop cannot pass through Apple Silicon
GPUs, so Macs use the `cpu` profile. Both images are multi-arch, so the CPU
profile runs natively on arm64 rather than under emulation.

## Your files

Notebooks live in the `pytorch-work` volume, mounted at `/home/jovyan/work` —
the `work` folder you see in JupyterLab. It survives `spin down` and is
deleted by `spin destroy`.

To keep notebooks on the host instead, replace the volume in `compose.yaml`:

```yaml
    volumes:
      - ${HOME}/notebooks:/home/jovyan/work
```

## Gotchas

- The old stack set `runtime: nvidia` unconditionally, so it failed to start on
  every machine without the NVIDIA toolkit — including all Macs. GPU access is
  now opt-in via the `gpu` profile and uses the modern `deploy.resources`
  syntax rather than the deprecated `runtime:` key.
- The old stack also ran Jupyter with `--NotebookApp.token=''` on `0.0.0.0`,
  leaving unauthenticated remote code execution open to anything that could
  reach the port. The token is mandatory here.
- The old `workspace:` named volume was declared but never mounted, so the
  bind-mounted `./workspace` was the only thing doing anything. Both are
  replaced by one named volume.
- These images are large — several GB — so the first `spin up pytorch` spends
  most of its time pulling.
- `pip install` inside the container does **not** survive a container rebuild
  unless the package lands under `/home/jovyan/work`. For a reproducible
  environment, add a `requirements.txt` to your work folder and reinstall, or
  build your own image `FROM` this one.
