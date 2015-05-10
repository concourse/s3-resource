# S3 Resource

Versions objects in an S3 bucket, by pattern-matching filenames to identify
version numbers.

## Source Configuration

* `access_key_id`: *Required.* The AWS access key to use when accessing the
bucket.

* `secret_access_key`: *Required.* The AWS secret key to use when accessing
the bucket.

* `region_name`: *Optional.* The region the bucket is in. Defaults to
`us-east-1`.

* `bucket`: *Required.* The name of the bucket.

* `regexp`: *Required.* The pattern to match filenames against. The first
grouped match is used to extract the version, or if a group is explicitly
named `version`, that group is used.

 The version extracted from this pattern is used to version the resource.
Semantic versions, or just numbers, are supported.

* `private`: *Optional.* Indicates that the bucket is private, so that any
URLs provided are signed.

* `cloudfront_url`: *Optional.* The URL (scheme and domain) of your CloudFront
distribution that is fronting this bucket. This will be used in the `url` file
that is given to the following task.

* `endpoint`: *Optional.* Custom endpoint for using S3 compatible provider.

* `disable_md5_hash_check`: *Optional.* Disables MD5 hash checking of files while uploading/downloading.

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

* `from`: *Required.* A regexp specifying the file to upload. If the regexp
matches more than one file, the output fails.

* `to`: *Optional.* A destination directory in the bucket (if ends with trailing slash), or destination filename in the bucket.
