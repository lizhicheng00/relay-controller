ALTER TABLE tunnel
    ADD UNIQUE KEY uk_tunnel_namespace_name (namespace, name);
