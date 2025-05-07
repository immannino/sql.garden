-- roles_permissions.sql

-- Define available roles
CREATE TABLE roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role_name TEXT UNIQUE NOT NULL,         -- e.g., 'admin', 'editor', 'viewer', 'member'
    description TEXT
);

-- Define available permissions
CREATE TABLE permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    permission_name TEXT UNIQUE NOT NULL,   -- e.g., 'create_post', 'edit_user', 'view_settings'
    description TEXT
);

-- Map users to roles (many-to-many)
CREATE TABLE user_roles (
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);

-- Map roles to permissions (many-to-many)
CREATE TABLE role_permissions (
    role_id INTEGER NOT NULL,
    permission_id INTEGER NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

-- Example query to get permissions for a user:
-- SELECT p.permission_name
-- FROM permissions p
-- JOIN role_permissions rp ON p.id = rp.permission_id
-- JOIN user_roles ur ON rp.role_id = ur.role_id
-- WHERE ur.user_id = ?; -- Replace ? with the user's ID
