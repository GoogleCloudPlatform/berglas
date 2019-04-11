// Copyright 2019 The Berglas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fn

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	_ "github.com/GoogleCloudPlatform/berglas/auto"
)

func F(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "API_KEY=%s\n", os.Getenv("API_KEY"))
	fmt.Fprintf(w, "TLS_KEY=%s\n", os.Getenv("TLS_KEY"))
	fmt.Fprintf(w, "\n")

	b, err := ioutil.ReadFile(os.Getenv("TLS_KEY"))
	if err != nil {
		fmt.Fprintf(w, "err reading file contents: %s\n", err)
	} else {
		fmt.Fprintf(w, "file contents: %s\n", b)
	}
}
