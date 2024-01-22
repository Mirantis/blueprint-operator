# Creating a release

The release process is automated using github actions that trigger when a release is created on the github page.

1. Open releases on the github page
2. Create a pre-release which includes
  a. A tag for the latest commit on main. Use semantic versioning: `X.Y.Z`
  b. The auto generated changelog
  c. Check the pre-release box
  d. Publish the release
3. CI will trigger and begin the release process
  a. Run through all tests (lint, unit, integration)
  b. Build the release images
  c. Publish the release images to
    i. ghcr.io/mirantiscontainers/boundless-operator:<tag>
    ii. ghcr.io/mirantiscontainers/boundless-operator:latest
    iii. ghcr.io/mirantiscontainers/boundless-operator:<commit SHA>
4. Once CI finished, take a look at the images and make sure they look good
5. Change the release from pre-release to latest on the github page
