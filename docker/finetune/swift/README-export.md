# Hugging Face Export for Fine-tuned Models

This document explains how to export fine-tuned models to Hugging Face Hub after completing the fine-tuning process using the Hugging Face API.

## Overview

The fine-tuning workflow now supports automatic export to Hugging Face Hub using direct API calls. After fine-tuning completes successfully, the model can be automatically exported and pushed to your Hugging Face repository without requiring CLI login.

## Environment Variables

To enable Hugging Face export, set the following environment variables:

### Required Variables

- `EXPORT_TO_HF`: Set to `"true"` to enable export to Hugging Face Hub
- `HF_USERNAME`: Your Hugging Face username
- `HF_MODEL_NAME`: The name for your model on Hugging Face Hub
- `HF_TOKEN`: Your Hugging Face API token (with write permissions)
- `HF_ENDPOINT`: The Hugging Face Hub endpoint URL (e.g., `https://huggingface.co`)

### Optional Variables

- `HF_REPO_NAME`: Repository name on Hugging Face Hub (defaults to `HF_MODEL_NAME`)
- `HF_COMMIT_MESSAGE`: Commit message for the export (defaults to "Fine-tuned model exported from ms-swift")
- `EXPORT_DIR`: Directory containing the fine-tuned model (defaults to `/workspace/output`)
- `MODEL_TYPE`: Model type for export (auto-detected from MODEL_ID if not provided)

## Usage

### 1. Set Environment Variables

```bash
export EXPORT_TO_HF="true"
export HF_USERNAME="your-username"
export HF_MODEL_NAME="my-finetuned-model"
export HF_TOKEN="hf_your_api_token_here"
export HF_ENDPOINT="https://huggingface.co"
```

### 2. Run Fine-tuning with Export

The fine-tuning process will automatically export to Hugging Face Hub if `EXPORT_TO_HF` is set to `"true"`:

```bash
# The start-job.sh script will handle both fine-tuning and export
./start-job.sh
```

### 3. Manual Export (Optional)

If you want to export a previously fine-tuned model manually:

```bash
# Set the required environment variables
export HF_USERNAME="your-username"
export HF_MODEL_NAME="my-finetuned-model"
export HF_TOKEN="hf_your_api_token_here"
export HF_ENDPOINT="https://huggingface.co"

# Run the export script
./export-to-csg.sh
```

## Workflow

1. **Fine-tuning**: The `swift sft` command runs with your specified parameters
2. **Success Check**: The script verifies that fine-tuning completed successfully
3. **Export Check**: If `EXPORT_TO_HF="true"`, the export process begins
4. **Repository Creation**: Uses Hugging Face API to create the repository if it doesn't exist
5. **Model Export**: Uses `swift export` with API credentials to convert and push the model to Hugging Face Hub
6. **Completion**: The model is now available at your specified HF_ENDPOINT

## Error Handling

The script includes comprehensive error handling:

- Validates that all required environment variables are set (including HF_ENDPOINT)
- Checks if the fine-tuning process completed successfully
- Verifies that the export directory exists
- Handles API authentication and repository creation failures
- Provides clear error messages for troubleshooting

## Security Notes

- Store your Hugging Face API token securely
- Use environment variables or secure secret management systems
- Ensure your API token has the necessary write permissions
- Consider using fine-grained tokens for better security

## Troubleshooting

### Common Issues

1. **Authentication Failed**: Verify your `HF_TOKEN` is correct and has write permissions
2. **Repository Already Exists**: The script will handle existing repositories gracefully
3. **Export Directory Not Found**: Ensure fine-tuning completed successfully before export
4. **Permission Denied**: Check that your Hugging Face token has write access to the repository
5. **API Endpoint Issues**: Verify that `HF_ENDPOINT` is correctly set and accessible

### Debug Mode

To see detailed output, you can modify the export script to include more verbose logging or run it manually to debug issues.

## Example

```bash
# Complete example with all environment variables
export EXPORT_TO_HF="true"
export HF_USERNAME="johndoe"
export HF_MODEL_NAME="my-llama-finetune"
export HF_REPO_NAME="my-llama-finetune-v1"
export HF_COMMIT_MESSAGE="Fine-tuned Llama model for specific task"
export HF_TOKEN="hf_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export HF_ENDPOINT="https://huggingface.co"
export MODEL_ID="meta-llama/Llama-2-7b-hf"
export DATASET_ID="my-dataset"
export EPOCHS="3"

# Run the complete workflow
./start-job.sh
```

After completion, your model will be available at:
`https://huggingface.co/johndoe/my-llama-finetune-v1`
