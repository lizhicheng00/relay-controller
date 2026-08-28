CREATE TEMPORARY TABLE tunnel_name_renames (
    _id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
    new_name VARCHAR(128) NOT NULL
);

LOCK TABLES
    tunnel WRITE,
    tunnel AS duplicate_row WRITE,
    tunnel AS retained_row READ,
    tunnel AS target_row WRITE,
    tunnel_name_renames WRITE;

INSERT INTO tunnel_name_renames (_id, new_name)
SELECT duplicate_row._id,
       CONCAT(
           LEFT(duplicate_row.name, 127 - CHAR_LENGTH(COUNT(retained_row._id) + 1)),
           '-',
           COUNT(retained_row._id) + 1
       )
FROM tunnel AS duplicate_row
JOIN tunnel AS retained_row
  ON retained_row.namespace = duplicate_row.namespace
 AND retained_row.name = duplicate_row.name
 AND (
      retained_row.deleted < duplicate_row.deleted
      OR (
          retained_row.deleted = duplicate_row.deleted
          AND retained_row._id < duplicate_row._id
      )
 )
GROUP BY duplicate_row._id, duplicate_row.name;

UPDATE tunnel AS target_row
JOIN tunnel_name_renames AS rename_row ON rename_row._id = target_row._id
SET target_row.name = rename_row.new_name;

ALTER TABLE tunnel
    ADD UNIQUE KEY uk_tunnel_namespace_name (namespace, name),
    LOCK=EXCLUSIVE;

UNLOCK TABLES;
DROP TEMPORARY TABLE tunnel_name_renames;
