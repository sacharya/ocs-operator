#!/bin/bash

source hack/common.sh

$OUTDIR_BIN/functests --ocs-registry-image="${FULL_IMAGE_NAME}" --local-storage-registry-image="${LOCAL_STORAGE_IMAGE_NAME}" --upgrade-to-ocs-registry-image="${UPGRADE_TO_OCS_REGISTRY_IMAGE}" --upgrade-to-local-storage-registry-image="${UPGRADE_TO_LOCAL_STORAGE_REGISTRY_NAME}" $@
if [ $? -ne 0 ]; then
	hack/dump-debug-info.sh
	echo "ERROR: Functest failed."
	exit 1
fi

