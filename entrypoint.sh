#!/bin/bash
# Dynamic user creation entrypoint for Docker containers
# Solves the "I have no name!" issue when running with --user UID:GID

set -e

# Get current UID and GID
CURRENT_UID=$(id -u)
CURRENT_GID=$(id -g)

# Check if the current UID already has a name in /etc/passwd
if ! getent passwd "$CURRENT_UID" >/dev/null 2>&1; then
	# Create group if it doesn't exist
	if ! getent group "$CURRENT_GID" >/dev/null 2>&1; then
		groupadd -g "$CURRENT_GID" dynamicgroup 2>/dev/null || true
	fi

	# Create user with the current UID/GID
	# Use the existing group name if it exists
	GROUP_NAME=$(getent group "$CURRENT_GID" | cut -d: -f1)
	if [ -z "$GROUP_NAME" ]; then
		GROUP_NAME="dynamicgroup"
	fi

	useradd -u "$CURRENT_UID" -g "$CURRENT_GID" -M -N -s /bin/bash dynamicuser 2>/dev/null || true

	# Ensure HOME directory exists (it's usually mounted from host or tmpfs)
	if [ -n "$HOME" ] && [ ! -d "$HOME" ]; then
		mkdir -p "$HOME" 2>/dev/null || true
	fi
fi

# Execute the actual command
exec "$@"
