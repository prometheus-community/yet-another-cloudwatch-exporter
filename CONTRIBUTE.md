# CONTRIBUTE

## Steps to Contribute

* We use [golangci-lint](https://github.com/golangci/golangci-lint) for linting the code. Make it sure to install it first.
* Check out repository running `git clone https://github.com/prometheus-community/yet-another-cloudwatch-exporter.git`
* For linting, please run `make lint`
* For building, please run `make build`
* For running locally, please run `./yace`
* Best practices:
  * commit should be as small as possible
  * branch from the *master* branch
  * add tests relevant to the fixed bug or new feature

## How to release
* `git tag v0.13.1-alpha && git push --tags`
