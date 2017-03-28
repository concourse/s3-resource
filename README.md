# S3 Resource

Versions objects in an S3 bucket, by pattern-matching filenames to identify
version numbers.

## Source Configuration

* `bucket`: *Required.* The name of the bucket.

* `access_key_id`: *Optional.* The AWS access key to use when accessing the
  bucket.

* `secret_access_key`: *Optional.* The AWS secret key to use when accessing
  the bucket.

* `region_name`: *Optional.* The region the bucket is in. Defaults to
  `us-east-1`.

* `private`: *Optional.* Indicates that the bucket is private, so that any
  URLs provided are signed.

* `cloudfront_url`: *Optional.* The URL (scheme and domain) of your CloudFront
  distribution that is fronting this bucket (e.g
  `https://d5yxxxxx.cloudfront.net`).  This will affect `in` but not `check`
  and `put`. `in` will ignore the `bucket` name setting, exclusively using the
  `cloudfront_url`.  When configuring CloudFront with versioned buckets, set
  `Query String Forwarding and Caching` to `Forward all, cache based on all` to
  ensure S3 calls succeed.

* `endpoint`: *Optional.* Custom endpoint for using S3 compatible provider.

* `disable_ssl`: *Optional.* Disable SSL for the endpoint, useful for S3
  compatible providers without SSL.

* `server_side_encryption`: *Optional.* An encryption algorithm to use when
  storing objects in S3.

* `sse_kms_key_id`: *Optional.* The ID of the AWS KMS master encryption key
  used for the object.


* `use_v2_signing`: *Optional.* Use signature v2 signing, useful for S3 compatible providers that do not support v4.

### File Names

One of the following two options must be specified:

* `regexp`: *Optional.* The pattern to match filenames against within S3. The first
  grouped match is used to extract the version, or if a group is explicitly
  named `version`, that group is used. At least one capture group must be
  specified, with parentheses.

  The version extracted from this pattern is used to version the resource.
  Semantic versions, or just numbers, are supported. Accordingly, full regular
  expressions are supported, to specify the capture groups.

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

Given a file specified by `file`, upload it to the S3 bucket. If `regexp` is
specified, the new file will be uploaded to the directory that the regex
searches in. If `versioned_file` is specified, the new file will be uploaded as
a new version of that file.

#### Parameters

* `file`: *Required.* Path to the file to upload, provided by an output of a task.
  If multiple files are matched by the glob, an error is raised. The file which
  matches will be placed into the directory structure on S3 as defined in `regexp`
  in the resource definition. The matching syntax is bash glob expansion, so
  no capture groups, etc.

* `acl`: *Optional.*  [Canned Acl](http://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html)
  for the uploaded object.
  
* `content_type`: *Optional.* MIME [Content-Type](https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.17)
  describing the contents of the uploaded object

## Example Configuration

### Resource

``` yaml
- name: release
  type: s3
  source:
    bucket: releases
    regexp: directory_on_s3/release-(.*).tgz
    access_key_id: ACCESS-KEY
    secret_access_key: SECRET
```

### Plan

``` yaml
- get: release
```

``` yaml
- put: release
  params:
    file: path/to/release-*.tgz
    acl: public-read
```

## Required IAM Permissions

### Non-versioned Buckets

The bucket itself (e.g. `"arn:aws:s3:::your-bucket"`):
* `s3:ListBucket`

The objects in the bucket (e.g. `"arn:aws:s3:::your-bucket/*"`):
* `s3:PutObject`
* `s3:PutObjectAcl`
* `s3:GetObject`

### Versioned Buckets

Everything above and...

The bucket itself (e.g. `"arn:aws:s3:::your-bucket"`):
* `s3:ListBucketVersions`
* `s3:GetBucketVersioning`

The objects in the bucket (e.g. `"arn:aws:s3:::your-bucket/*"`):
* `s3:GetObjectVersion`
* `s3:PutObjectVersionAcl`

## Developing on this resource

First get the resource via:
`go get github.com/concourse/s3-resource`


