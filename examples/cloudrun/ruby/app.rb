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

require 'sinatra'

set :bind, '0.0.0.0'
set :port, ENV['PORT'] || '8080'

configure :production do
  require 'stackdriver'
  use Google::Cloud::Logging::Middleware
  use Google::Cloud::ErrorReporting::Middleware
  use Google::Cloud::Trace::Middleware
  use Google::Cloud::Debugger::Middleware
end

get '/' do
  api_key = ENV['API_KEY']
  tls_key_path = ENV['TLS_KEY']

  tls_key = "file does not exist"
  if !tls_key_path.nil? && File.exist?(tls_key_path)
    tls_key = File.read(tls_key_path)
  end

  resp =  "API_KEY: #{api_key}\n"
  resp << "TLS_KEY_PATH: #{tls_key_path}\n"
  resp << "TLS_KEY: #{tls_key}\n"
  resp
end
