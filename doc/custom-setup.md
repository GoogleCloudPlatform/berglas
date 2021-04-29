# Berglas Custom Setup

This document describes the steps required to create a Berglas Cloud Storage
bucket and Cloud KMS keys manually. **This is an advanced user topic and is not
required to use Berglas. Users should use the `berglas bootstrap` command where
possible!**

1. Install the [Cloud SDK][cloud-sdk]. More detailed instructions are available
   in the main README.

1. Export your project ID as an environment variable. The rest of this setup
guide assumes this environment variable is set:

    ```text
    export PROJECT_ID=my-gcp-project-id
    ```

    Please note, this is the project _ID_, not the project _name_ or project
    _number_. You can find the project ID by running `gcloud projects list` or
    in the web UI.

1. Enable required services on the project:

    ```text
    gcloud services enable --project ${PROJECT_ID} \
      cloudkms.googleapis.com \
      storage-api.googleapis.com \
      storage-component.googleapis.com
    ```

1. Create a [Cloud KMS][cloud-kms] keyring and crypto key for encrypting
secrets:

    ```text
    gcloud kms keyrings create my-keyring \
      --project ${PROJECT_ID} \
      --location global
    ```

    ```text
    gcloud kms keys create my-key \
      --project ${PROJECT_ID} \
      --location global \
      --keyring my-keyring \
      --purpose encryption
    ```

    You can choose alternate locations and names, but the `purpose` must remain
    as "encryption".

1. Create a [Cloud Storage][cloud-storage] bucket for storing secrets:

    ```text
    export BUCKET_ID=my-secrets
    ```

    Replace `my-secrets` with the name of your bucket. Bucket names must be
    globally unique across all of Google Cloud. You can also create a bucket
    using the Google Cloud Console from the web.

    ```text
    gsutil mb -p ${PROJECT_ID} gs://${BUCKET_ID}
    ```

    **It is strongly recommended that you create a new bucket instead of using
    an existing one. Berglas should be the only entity managing IAM permissions
    on the bucket.**

1. Set the default ACL permissions on the bucket to private:

    ```text
    gsutil defacl set private gs://${BUCKET_ID}
    ```

    ```text
    gsutil acl set private gs://${BUCKET_ID}
    ```

    The default permissions grant anyone with Owner/Editor access on the project
    access to the bucket and its objects. These commands restrict access to the
    bucket to project owners and access to bucket objects to only their owner.
    Everyone else must be granted explicit access via IAM to an object inside
    the bucket.
