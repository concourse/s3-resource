⚠️  DO NOT MERGE. STILL TESTING

# Why

When the `s3-resource` pulls an archive whose name changes (for instance, depending on a version number), it is hard to use the extracted content in a task after extracting it with `unpack: true`. This is because the archive's content is extracted into a directory whose name is the base name of the archive, which is dynamic. As far as I can tell, Concourse's `job.plan.task.file` doesn't allow dynamic values, so using this resource to pull down and extract task files is cumbersome.

Let me explain.

# Example

### The S3 bucket
Imagine you had an s3 bucket called `my-tasks-bucket` with these files:
- `my-tasks-v2.tar.gz`
-  `my-tasks-v1.tar.gz`

Imagine further that each of these archives contains two tasks, `task-a.yml` and `task-b.yml`.

### The resource definition
Imagine that you defined an s3 resource to get files from that bucket
```yaml
- name: my-s3-tasks
  type: s3
  source:
    bucket: my-tasks-bucket
    regexp: my-tasks-(.*).tar.gz
```

### Using the resource in a task
Assume we want to use `task-a.yml` in a job. Naturally, you'd `get` the resource (with `unpack: true`) and use the task in the job's plan. It is not that simple.

```yaml
jobs:
- name: use-task-from-s3
  plan:
  - get: my-s3-tasks
    params: {unpack: true}
  - task: task-a
    file: my-s3-tasks/my-tasks-<???>/task-a.yml 
   # We don't know the version because it is dynamic. Read on for details. 
```

We can't determine the directory into which `task-a.yml` will be extracted because the name of the archive file pulled by the s3 resource changes depending on the latest version available in the bucket. One day it could be `my-tasks-v1.tar.gz` and on another day, be `my-tasks-v2.tar.gz`.  
> Note that `unpack: true` extracts the archive into a directory named after the basename of the archive file. So `my-tasks-v1.tar.gz`'s files will be extracted into `my-s3-tasks/my-tasks-v1`

I tried to parameterize the version in the task definition like so `file: my-s3-bucket/my-tasks-((version))/task-a.yml` but Concourse doesn't like that (doesn't seem to hydrate values in `task.file` definitions. But even if I could, it was a hack.

# What
1. Introduce a new get param named `unpack_into` (expects string). This allows the user to set the path into which the archive will be extracted, making the extracted file location predictable and hence much easier to use in a task.
2. Make it so that if you set `unpack_into` to a non-empty string, `unpack` will be set to true. This is for two reasons: 
    - `unpack_into` doesn't make sense without `unpack: true`
    - having to set both parameters every time you need to unpack into a custom dir is cumbersome. The name `unpack_into` makes it very clear that unpacking will happen, so it is a bit redundant to add another unpack instruction.

## The result
This should allow me to:

```yaml
- name: my-s3-tasks
  type: s3
  source:
    bucket: my-tasks-bucket
    regexp: my-tasks-(.*).tar.gz

jobs:
- name: use-task-from-s3
  plan:
  - get: my-s3-tasks
    params:
      unpack_into: extracted
  - task: scan
    file: my-s3-tasks/extracted/task-a.yml
```
> The use of `unpack_into: extracted`  "stabilizes" the extraction dir, making it predictable.

One could also extract into the resource dir, making the `task.file` usage even cleaner. (this could be dangerous because it has the potential to overwrite the default files the task puts in the input directory like `version`). It would look something like 
```yaml
jobs:
- name: use-task-from-s3
  plan:
  - get: my-s3-tasks
    params:
      unpack_into: . # extract directly into my-s3-tasks dir
  - task: scan
    file: my-s3-tasks/task-a.yml
```