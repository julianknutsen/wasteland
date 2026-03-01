package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/inference"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newInferPostCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		prompt    string
		model     string
		seed      int
		maxTokens int
		noPush    bool
	)

	cmd := &cobra.Command{
		Use:   "post",
		Short: "Post a new inference job to the wanted board",
		Long: `Post a new LLM inference job to the Wasteland wanted board.

Creates a wanted item with type=inference and the job parameters (prompt,
model, seed) encoded as JSON in the description field.

Best-effort check that the model exists in the local ollama instance.

Examples:
  wl infer post --prompt "what is 1+1" --model llama3.2:1b
  wl infer post --prompt "explain gravity" --model llama3.2:1b --seed 123 --max-tokens 200
  wl infer post --prompt "hello" --model mistral:7b --no-push`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInferPost(cmd, stdout, stderr, prompt, model, seed, maxTokens, noPush)
		},
	}

	cmd.Flags().StringVar(&prompt, "prompt", "", "Inference prompt (required)")
	cmd.Flags().StringVar(&model, "model", "", "Ollama model tag, e.g. llama3.2:1b (required)")
	cmd.Flags().IntVar(&seed, "seed", 42, "Random seed for deterministic output")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Maximum tokens (0 = model default)")
	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")
	_ = cmd.MarkFlagRequired("prompt")
	_ = cmd.MarkFlagRequired("model")

	return cmd
}

func runInferPost(cmd *cobra.Command, stdout, _ io.Writer, prompt, model string, seed, maxTokens int, noPush bool) error {
	// Best-effort model existence check.
	if exists, err := inference.ModelExists(model); err == nil && !exists {
		fmt.Fprintf(stdout, "  %s model %q not found in local ollama (job will still be posted)\n",
			style.Warning.Render(style.IconWarn), model)
	}

	job := &inference.Job{
		Prompt:    prompt,
		Model:     model,
		Seed:      seed,
		MaxTokens: maxTokens,
	}

	description, err := inference.EncodeJob(job)
	if err != nil {
		return err
	}

	title := inferTitle(prompt)

	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
	}

	item := &commons.WantedItem{
		ID:          commons.GenerateWantedID(title),
		Title:       title,
		Description: description,
		Type:        "inference",
		Priority:    2,
		PostedBy:    wlCfg.RigHandle,
		EffortLevel: "small",
	}

	mc := newMutationContext(wlCfg, item.ID, noPush, stdout)
	cleanup, err := mc.Setup()
	if err != nil {
		return err
	}
	defer cleanup()

	store := openStore(wlCfg.LocalDir, wlCfg.Signing, wlCfg.HopURI)

	if err := postWanted(store, item); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Posted inference job: %s\n", style.Bold.Render("✓"), style.Bold.Render(item.ID))
	fmt.Fprintf(stdout, "  Model:  %s\n", model)
	fmt.Fprintf(stdout, "  Seed:   %d\n", seed)
	fmt.Fprintf(stdout, "  Prompt: %s\n", truncate(prompt, 80))
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Next: wl infer run "+item.ID))

	return nil
}

// inferTitle generates a title from the prompt, truncating at 60 chars.
func inferTitle(prompt string) string {
	const maxLen = 60
	s := prompt
	if len(s) > maxLen {
		s = s[:maxLen] + "..."
	}
	return "infer: " + s
}

// truncate shortens a string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
