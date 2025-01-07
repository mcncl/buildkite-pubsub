# Google Cloud Setup Guide

This guide walks through setting up Google Cloud Project access for the Buildkite Webhook service. It covers creating a service account with the minimum required permissions.

## Prerequisites

- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) installed
- Access to a Google Cloud Project with owner or security admin permissions
- Project ID where you want to deploy the webhook service

## Service Account Setup

### 1. Set Environment Variables

```bash
# Set your project ID
export PROJECT_ID="your-project-id"
```

### 2. Create Service Account

```bash
# Create a new service account for the webhook service
gcloud iam service-accounts create buildkite-webhook \
    --description="Service account for Buildkite webhook" \
    --display-name="Buildkite Webhook"
```

### 3. Grant Required Permissions

#### Option A: Using Predefined Roles (Recommended for most users)

```bash
# Grant Pub/Sub Publisher role
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/pubsub.publisher"

# Optional: Grant topic creation permissions
# Only needed if you want the service to create topics automatically
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/pubsub.topics.create"
```

#### Option B: Using Custom Role (More restrictive)

```bash
# Create a custom role with minimal permissions
gcloud iam roles create buildkite_webhook \
    --project=$PROJECT_ID \
    --title="Buildkite Webhook" \
    --description="Custom role for Buildkite webhook service" \
    --permissions="pubsub.topics.publish,pubsub.topics.get"

# Assign the custom role to the service account
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="projects/$PROJECT_ID/roles/buildkite_webhook"
```

### 4. Create and Download Credentials

```bash
# Create and download the service account key
gcloud iam service-accounts keys create credentials.json \
    --iam-account=buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com
```

### 5. Verify Setup

```bash
# List service account's IAM policy bindings
gcloud projects get-iam-policy $PROJECT_ID \
    --flatten="bindings[].members" \
    --format='table(bindings.role)' \
    --filter="bindings.members:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com"

# Verify the credentials file exists
ls -l credentials.json
```

## Security Considerations

1. **Key Rotation**: Consider implementing regular key rotation for the service account
2. **Minimal Permissions**: The custom role option provides the most restrictive set of permissions
3. **Key Storage**: Store the credentials.json file securely and never commit it to version control

## Common Issues

1. **Permission Denied Errors**:
   - Verify the service account has the correct roles assigned
   - Check that the credentials file is mounted correctly in the pod
   - Ensure the PROJECT_ID matches the one where permissions were granted

2. **Missing Permissions**:
   ```bash
   # Check effective permissions for the service account
   gcloud iam service-accounts get-iam-policy \
       buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com
   ```

3. **Invalid Credentials**:
   - Verify the credentials file is valid JSON
   - Check the file permissions in the pod
   - Ensure GOOGLE_APPLICATION_CREDENTIALS is set correctly

## Next Steps

After completing this setup:

1. Store the credentials.json file securely
2. Follow the [Quick Start Guide](QUICK_START.md) to deploy the webhook service
3. Consider setting up monitoring for the service account's activity

## Cleanup

If you need to remove the service account:

```bash
# Delete the service account
gcloud iam service-accounts delete \
    buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com

# If you created a custom role, delete it
gcloud iam roles delete buildkite_webhook --project=$PROJECT_ID

# Remove the credentials file
rm credentials.json
```
