# Contributing to Berglas

We'd love to accept your patches and contributions to this project. There are
just a few small guidelines you need to follow.

## Community Guidelines

This project follows
[Google's Open Source Community Guidelines](https://opensource.google.com/conduct/).

## Contributor License Agreement

Contributions to this project must be accompanied by a Contributor License
Agreement. You (or your employer) retain the copyright to your contribution;
this simply gives us permission to use and redistribute your contributions as
part of the project. Head over to <https://cla.developers.google.com/> to see
your current agreements on file or to sign a new one.

You generally only need to submit a CLA once, so if you've already submitted one
(even if it was for a different project), you probably don't need to do it
again.

## Code reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.


## Testing

If you want to develop on Berglas, you will need to create a test setup.

1. Create a dedicated Google Cloud project in which to run the tests. No other
infrastructure or services should run in this project.

    ```text
    $ gcloud projects create $PROJECT_ID
    ```

1. Enable the necessary APIs on the project:

    ```text
    $ gcloud services enable --project $PROJECT_ID \
        cloudkms.googleapis.com \
        storage-api.googleapis.com \
        storage-component.googleapis.com
    ```

1. Create a Cloud Storage bucket:

    ```text
    $ gsutil mb -p $PROJECT_ID gs://$BUCKET_ID
    ```

1. Create a Cloud KMS key:

    ```text
    $ gcloud kms keyrings create my-keyring \
        --project $PROJECT_ID \
        --location global

    $ gcloud kms keys create my-key \
        --project $PROJECT_ID \
        --location global \
        --keyring my-keyring \
        --purpose encryption
    ```

1. Export the values as environment variables:

    ```text
    $ export GOOGLE_CLOUD_PROJECT=$PROJECT_ID
    $ export GOOGLE_CLOUD_BUCKET=$BUCKET_ID
    $ export GOOGLE_CLOUD_KMS_KEY=projects/$PROJECT_ID/locations/global/keyRings/my-keyring/cryptoKeys/my-key
    ```

1. Run tests:

    ```text
    $ make test-acc
    ```
