# Copyright 2019 The Berglas Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

require 'open3'

module Berglas
  class Error < StandardError; end

  # Prefix for the reference format.
  PREFIX = 'berglas://'.freeze

  extend self

  # Returns if this ref is a berglas reference.
  #
  # @param ref [String] the ref to check
  #
  # @return [true, false] whether the ref is a berglas reference
  def reference?(ref)
    return ref.start_with?(PREFIX)
  end

  # Parse berglas references from the local environment and replace them with
  # their plaintext values in this processes' memory.
  #
  # @param local [true, false] if true, parse the local ENV hash; if false,
  # communicate with the upstream cloud service and determine the list of
  # environment variable that were set on the resource at deploment.
  #
  # @return [nil]
  def auto(local = false)
    refs = ENV.select { |_, v| self.reference?(v) }
    out = berglas('exec', "--local=#{local}", 'env')
    resolved = Hash[*out.split("\n").map { |v| v.split("=", 2) }.flatten]
    refs.each { |k, _| ENV[k] = resolved[k] }
    return nil
  end

  # Accesses a secret's material.
  #
  # @param secret [String] name of the secret including the bucket name
  #
  # @return [String] contents of the secret
  def access(secret)
    return berglas('access', secret)
  end

  # Create a new storage bucket and KMS key for storing secrets.
  #
  # @param project [String] ID of the project where the bucket and keys should reside
  # @param bucket [String] name of the bucket to create
  # @param bucket_location [String] location to create the bucket
  # @param kms_location [String] location to create the kms keys
  # @param kms_keyring [String] keyring to create the kms keys
  # @param kms_key [String] key to create in kms
  #
  # @return [nil]
  def bootstrap(
    project:,
    bucket:,
    bucket_location: 'US',
    kms_location: 'global',
    kms_keyring: 'berglas',
    kms_key: 'berglas-key'
  )

    berglas('bootstrap',
      '--project', project,
      '--bucket', bucket,
      '--bucket-location', bucket_location,
      '--kms-location', kms_location,
      '--kms-keyring', kms_keyring,
      '--kms-key', kms_key,
    )
    return nil
  end

  # Add or update a secret with the given data.
  #
  # @param secret [String] name of the secret
  # @param data [String, #read] data to put into the secret
  # @param key [String] ID of the KMS key
  #
  # @return [nil]
  def create(secret, data, key)
    data = data.read if data.respond_to?(:read)
    berglas('create', secret, data, '--key', key)
    return nil
  end

  # Delete the secret and its contents.
  #
  # @param secret [String] name of the secret
  #
  # @return [nil]
  def delete(secret)
    berglas('delete', secret)
    return nil
  end

  # Grant access to the secret to the given members. Members should be in the
  # format defined by Google IAM (i.e. prefixed with "user" or "serviceAccount",
  # etc).
  #
  # @param secret [String] name of the secret
  # @param members [Array<String>] list of members to grant access
  #
  # @return [nil]
  def grant(secret, *members)
    return nil if members.empty?
    berglas('grant', secret, *['--member'].product(members).flatten)
    return nil
  end

  # Show all secret names in a bucket.
  #
  # @param bucket [String] name of the bucket
  #
  # @return [Array<String>] list of secret names
  def list(bucket)
    list = berglas('list', bucket)
    list = list.split("\n") unless list.nil?
    return list
  end

  # Revoke access to the secret to the given members. Members should be in the
  # format defined by Google IAM (i.e. prefixed with "user" or "serviceAccount",
  # etc).
  #
  # @param secret [String] name of the secret
  # @param members [Array<String>] list of members to revoke access
  #
  # @return [nil]
  def revoke(secret, *members)
    return nil if members.empty?
    berglas('revoke', secret, *['--member'].product(members).flatten)
    return nil
  end

  private

  def berglas(*args)
    out, status = Open3.capture2e(berglas_bin, *args)
    raise Error.new(out) if !status.success?
    return out
  end

  def berglas_bin
    @berglas_bin ||= File.expand_path(File.join(File.dirname(__FILE__), '../ext/berglas'))
  end
end
