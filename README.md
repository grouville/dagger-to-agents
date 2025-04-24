# From 0 to hero in agentic UX, using Dagger

This introduces a nice 0 -> hero gradual build-up on the agentic use of Dagger

## Setup

## Running the evals

```shell
$ cd hello-dagger
$ export OPENAI_API_KEY=***
$ dagger_dev call --progress plain run-evals report
```

## Running the evals with Goose (NOT WORKING, WIP)

```shell
$ cd hello-dagger
$ export OPENAI_API_KEY=***
$ dagger call --progress plain run-evals --project . --llm-key env://OPENAI_API_KEY --dagger-cli $(which dagger)
```
