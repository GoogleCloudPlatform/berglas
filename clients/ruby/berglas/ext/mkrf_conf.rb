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

require 'open-uri'
require 'rbconfig'
include RbConfig

# URL is the download path
URL = 'https://storage.googleapis.com/berglas/master/%{os}_%{arch}/berglas'.freeze

# Check arch
arch = CONFIG['host_cpu']
if arch =~ /32/
  raise 'Berglas is only supported on 64-bit operating systems'
end
arch = 'amd64'

# Check os
os = nil
case CONFIG['host_os']
when /mswin|msys|mingw|cygwin|bccwin|wince|emc/
  os = 'windows'
when /darwin|mac os/
  os = 'darwin'
when /linux/
  os = 'linux'
else
  raise "Berglas is not supported on your system: #{CONFIG['host_os']}"
end

url = URL % { os: os, arch: arch }
dest = File.join(Dir.pwd, 'berglas')

puts "detected #{os}_#{arch}"
puts "downloading from #{url}"
puts "saving to #{dest}"

# Download the platform-specific berglas build
File.open(dest, 'wb') do |f|
  open(url, 'rb') { |r| f.write(r.read) }
end
File.chmod(0775, dest)

puts "berglas is installed at #{dest}"

# Signal success
path = File.join(File.dirname(__FILE__), 'Rakefile')
File.write(path, "task :default\n", mode: 'w')
