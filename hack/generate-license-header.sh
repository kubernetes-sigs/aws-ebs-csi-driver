#!/bin/bash

set -euo pipefail

echo "Adding license header..."

current_year=$(date +%Y)
find . -type d \( -name "deploy" -o -name "charts" \) -prune -o -type f \( -name "*.go" -o -name "*.sh" -o -name "*.yaml" -o -name "*.yml" \) -print | while read -r file; do
  case "$file" in
  *.go)
    comment_prefix="//"
    ;;
  *.sh | *.yaml | *.yml)
    comment_prefix="#"
    ;;
  esac
  if ! grep -q "The Kubernetes Authors." "$file"; then
    echo -e "${comment_prefix} Copyright ${current_year} The Kubernetes Authors.\n${comment_prefix}\n${comment_prefix} Licensed under the Apache License, Version 2.0 (the 'License');\n${comment_prefix} you may not use this file except in compliance with the License.\n${comment_prefix} You may obtain a copy of the License at\n${comment_prefix}\n${comment_prefix}    http://www.apache.org/licenses/LICENSE-2.0\n${comment_prefix}\n${comment_prefix} Unless required by applicable law or agreed to in writing, software\n${comment_prefix} distributed under the License is distributed on an 'AS IS' BASIS,\n${comment_prefix} WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.\n${comment_prefix} See the License for the specific language governing permissions and\n${comment_prefix} limitations under the License.\n" | cat - "$file" >temp && mv temp "$file"
  fi
done
