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

# Note: if you switch to a slim base image, like an alpine-based image, you
# will need to add the ca-certificates package for Berglas to work.
FROM ruby:2.6

ENV APP_ENV=production

WORKDIR /usr/src/app

COPY Gemfile Gemfile.lock ./
RUN bundle install \
  --clean \
  --binstubs \
  --deployment \
  --frozen \
  --jobs 7 \
  --no-cache \
  --quiet

COPY . ./
COPY --from=gcr.io/berglas/berglas:latest /bin/berglas /bin/berglas

ENTRYPOINT exec /bin/berglas exec -- ./bin/bundle exec ruby app.rb
