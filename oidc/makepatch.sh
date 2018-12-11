kubectl get ds --namespace kube-system kube-apiserver -o json | tee apiserver.orig.json | \
jq --argjson commands "$(<command.json)" -r '.spec.template.spec.containers[0].command += $commands' | \
jq --argjson volumemount "$(<volumeMount.json)" -r '.spec.template.spec.containers[0].volumeMounts += [$volumemount]' | \
jq --argjson volume "$(<volume.json)" -r '.spec.template.spec.volumes += [$volume]' > patch.json
