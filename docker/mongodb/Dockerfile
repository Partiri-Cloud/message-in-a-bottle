# MongoDB image for message-in-a-bottle.
#
# Persistent storage is mounted at /app/data at runtime; mongod.conf points
# storage.dbPath there so all database files land on the mounted volume.
FROM mongo:8

# Custom config that relocates dbPath from the default /data/db to /app/data.
COPY mongod.conf /etc/mongod.conf

# Create the data directory so the path exists as a mount point. Ownership of
# the *mounted* volume is NOT set here: a build-time chown is overlaid (and thus
# ignored) once the persistent volume is mounted at runtime. The mounted volume
# must instead be made writable by the runtime user at deploy time — set the
# pod securityContext fsGroup to the runAsUser UID (1001 on this platform) so
# the kernel grants group ownership of the volume to mongod.
RUN mkdir -p /app/data

# Declare the persistent-storage mount point.
VOLUME /app/data

EXPOSE 27017

CMD ["mongod", "--config", "/etc/mongod.conf"]
