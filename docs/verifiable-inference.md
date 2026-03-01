# Verifiable Distributed Inference

Post LLM inference jobs to the wanted board, have workers run them
deterministically via ollama, and verify results by comparing SHA-256
hashes. This is trustless distributed compute — anyone can verify that a
claimed result was actually produced by the specified model and seed.

## Prerequisites

- `wl` CLI installed and joined to a wasteland (`wl join`)
- [Ollama](https://ollama.com) installed and running locally
- A model pulled: `ollama pull llama3.2:1b`

## How it works

1. **Poster** creates a wanted item with `type=inference` containing the
   job parameters (prompt, model, seed) as JSON in the description field.
2. **Worker** claims the item, runs it against their local ollama with
   deterministic settings (`temperature=0`, fixed seed), and submits the
   output plus its SHA-256 hash as completion evidence.
3. **Verifier** re-runs the same job locally and compares hashes. If the
   model is deterministic with the given seed, the hashes match.

The key insight: with `temperature=0` and a fixed seed, ollama produces
identical output for the same prompt and model. This makes the output
hash a verifiable proof of work.

## Commands

### `wl infer post` — post a job

```bash
wl infer post --prompt "what is 1+1" --model llama3.2:1b
```

Flags:
- `--prompt` (required) — the inference prompt
- `--model` (required) — ollama model tag (e.g. `llama3.2:1b`, `mistral:7b`)
- `--seed` — random seed for deterministic output (default: 42)
- `--max-tokens` — maximum tokens, 0 = model default
- `--no-push` — skip pushing to remotes

This creates a wanted item with `type=inference` and the job encoded as
JSON in the description. A best-effort check warns if the model isn't
found in your local ollama.

### `wl infer run <wanted-id>` — claim and execute

```bash
wl infer run w-abc123
wl infer run w-abc123 --skip-claim
```

Claims the item, decodes the job from the description, runs it via
ollama, and submits a completion with the result and SHA-256 hash as
evidence. If ollama fails, the claim is automatically released so
another worker can retry.

Use `--skip-claim` when the item was already claimed externally (e.g.,
by the wasteland-feeder automation). In this mode the item must have
`status=claimed` instead of `status=open`, and the claim is NOT released
on failure — the external claimer owns the claim lifecycle.

Flags:
- `--skip-claim` — skip claiming (item already claimed externally)
- `--no-push` — skip pushing to remotes

### `wl infer verify <wanted-id>` — re-run and compare

```bash
wl infer verify w-abc123
```

Re-runs the inference job locally and compares the output hash against
the submitted completion evidence. Prints VERIFIED (hashes match) or
MISMATCH (hashes differ) with both hashes for inspection.

### `wl infer status <wanted-id>` — inspect details

```bash
wl infer status w-abc123
```

Shows the standard wanted item status plus decoded inference metadata:
model, seed, prompt, and (if completed) the output hash and a truncated
output preview.

## End-to-end example

```bash
# Pull a small model
ollama pull llama3.2:1b

# Post a job
wl infer post --prompt "explain why the sky is blue in one sentence" \
  --model llama3.2:1b --seed 42 --no-push
# => Posted inference job: w-a1b2c3

# Worker picks it up
wl infer run w-a1b2c3 --no-push
# => Inference completed for w-a1b2c3

# Anyone can verify
wl infer verify w-a1b2c3
# => VERIFIED — hashes match

# Check the full status
wl infer status w-a1b2c3
```

## Data encoding

Job parameters and results are stored as JSON in the existing wanted
item text fields — no schema changes required.

**Description field** (job parameters):
```json
{
  "prompt": "what is 1+1",
  "model": "llama3.2:1b",
  "seed": 42,
  "max_tokens": 100
}
```

**Evidence field** (completion result):
```json
{
  "output": "The answer is 2.",
  "output_hash": "sha256:e3b0c44298fc...",
  "model": "llama3.2:1b",
  "seed": 42
}
```

## Determinism caveats

Ollama determinism depends on the model and hardware. Most models produce
identical output with `temperature=0` and a fixed seed on the same
architecture. Cross-architecture reproducibility (e.g. x86 vs ARM) is
not guaranteed for all models due to floating-point differences. When
verification fails, check that both parties are using the same model tag
and compatible hardware.
