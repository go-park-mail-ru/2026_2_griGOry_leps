package repository

const addressColumns = `address_id, user_id, street, house, comment, is_default, created_at`

const listAddressesQuery = `
 SELECT ` + addressColumns + `
 FROM address
 WHERE user_id=$1
 ORDER BY created_at, address_id
`

const countAddressesQuery = `SELECT count(*) FROM address WHERE user_id=$1`

const getAddressForUpdateQuery = `
 SELECT ` + addressColumns + `
 FROM address
 WHERE address_id=$1 AND user_id=$2
 FOR UPDATE
`

const createAddressQuery = `
 INSERT INTO address (user_id, street, house, comment, is_default)
 VALUES ($1, $2, $3, $4, $5)
 RETURNING ` + addressColumns

const updateAddressQuery = `
 UPDATE address
 SET street=$3, house=$4, comment=$5, is_default=$6
 WHERE address_id=$1 AND user_id=$2
 RETURNING ` + addressColumns

const deleteAddressQuery = `DELETE FROM address WHERE address_id=$1 AND user_id=$2`

const clearDefaultAddressQuery = `
 UPDATE address
 SET is_default=false
 WHERE user_id=$1 AND is_default
`

const promoteOldestAddressQuery = `
 UPDATE address
 SET is_default=true
 WHERE address_id = (
  SELECT address_id FROM address
  WHERE user_id=$1 AND address_id<>$2
  ORDER BY created_at, address_id
  LIMIT 1
 )
`
