# S3 Resource

Versions objects in an S3 bucket, by pattern-matching filenames to identify
version numbers.

## Source Configuration

* `access_key_id`: *Required.* The AWS access key to use when accessing the
  bucket.

* `secret_access_key`: *Required.* The AWS secret key to use when accessing
  the bucket.

* `bucket`: *Required.* The name of the bucket.

* `region_name`: *Optional.* The region the bucket is in. Defaults to
  `us-east-1`.

* `private`: *Optional.* Indicates that the bucket is private, so that any
  URLs provided are signed.

* `cloudfront_url`: *Optional.* The URL (scheme and domain) of your CloudFront
  distribution that is fronting this bucket. This will be used in the `url`
  file that is given to the following task.

* `endpoint`: *Optional.* Custom endpoint for using S3 compatible provider.

### File Names

One of the following two options must be specified:

* `regexp`: *Optional.* The pattern to match filenames against. The first
  grouped match is used to extract the version, or if a group is explicitly
  named `version`, that group is used.

  The version extracted from this pattern is used to version the resource.
  Semantic versions, or just numbers, are supported.

* `versioned_file`: *Optional* If you enable versioning for your S3 bucket then
  you can keep the file name the same and upload new versions of your file
  without resorting to version numbers. This property is the path to the file
  in your S3 bucket.

## Behavior

### `check`: Extract versions from the bucket.

Objects will be found via the pattern configured by `regexp`. The versions
will be used to order them (using [semver](http://semver.org/)). Each
object's filename is the resulting version.


### `in`: Fetch an object from the bucket.

Places the following files in the destination:

* `(filename)`: The file fetched from the bucket.

* `url`: A file containing the URL of the object. If `private` is true, this
  URL will be signed.

* `version`: The version identified in the file name.

#### Parameters

*None.*


### `out`: Upload an object to the bucket.

Given a path specified by `from`, upload it to the S3 bucket, optionally to
a directory configured by `to`. The path must identify a single file.

#### Parameters
  
* `file`: *Required.* Path to the file to upload. If multiple files are
  matched by the glob, an error is raised.

The following fields are deprecated and may be specified instead of `file`
(until we remove them, anyway):

* `from`: *Optional.* **Deprecated.** A regexp specifying the file to upload.
  If the regexp matches more than one file, the output fails.

* `to`: *Optional.* **Deprecated.** A destination directory in the bucket.
  If this ends in a "/" then the file will keep its name but be uploaded to
  that directory.

## Example Configuration

### Resource

``` yaml
- name: release
  type: s3
  source:
    bucket: releases
    regexp: release-name-(.*).tgz
    access_key_id: AKIA-ACCESS-KEY
    secret_access_key: SECRET
```

### Plan

``` yaml
- get: release
```

``` yaml
- put: release
  params:
    from: a/release/path/release-(.*).tgz
```

## Required IAM Permissions

### Non-versioned Buckets

* `s3:PutObject`
* `s3:GetObject`
* `s3:ListBucket`

### Versioned Buckets

Everything above and...

* `s3:GetBucketVersioning`
* `s3:GetObjectVersion`
* `s3:ListBucketVersions`
