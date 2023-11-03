# Workflows

There are two types of workflows in this directory. I would put them in subfolders but GitHub doesn't support that.

## Callers

These are the high level workflows that can be associated with what triggers them. PRs, releases, nightlys, merges, etc. These are made up of jobs that are defined the the other workflows. These are the workflows that you will see in the Actions tab of the repo. By grouping these tasks into parent workflows, the jobs are grouped under one action in the actions tab. They share the smaller 'job' workflows so that they always run the same way. Convention has become to capitalize the first letter of these workflow's name.

## Jobs

These are the smaller individual tasks that are used to build up the larger parent workflows. They can be thought of as running unit tests, building the binaries, or linting the code. When you open one of the parent caller actions in the actions tab, they will show these individual jobs. Convention has become to lowercase the first letter of these workflow's name.

# Working with workflows

The easiest way to test a workflow is by creating it on your forked repo. This way you have control over the settings and you can manipulate branches anyway you need to trigger the workflow. When testing this way, you should be careful that you are pushing to your repo and not the company's and also make sure to clean everything up in your repo once you have finished testing.
