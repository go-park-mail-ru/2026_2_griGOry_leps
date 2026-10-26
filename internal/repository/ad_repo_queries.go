package repository

const adCardColumns = `
 a.id,a.user_id,COALESCE(a.category_id,0),a.title,
 COALESCE(a.description,''),a.price,
 COALESCE(a.city,''),a.has_delivery,a.is_free,COALESCE(image.url,''),a.status,
 COALESCE(a.close_reason,''),a.created_at,a.updated_at
`

const adMainImageJoin = `
 LEFT JOIN LATERAL (
  SELECT url FROM ad_image WHERE ad_id=a.id ORDER BY position,image_id LIMIT 1
 ) image ON true
`

const sellerStatsJoin = `
 LEFT JOIN LATERAL (
  SELECT ROUND(AVG(rv.rating), 2)::float8 AS rating, COUNT(*) AS reviews_count
  FROM review rv
  JOIN ad ra ON ra.id = rv.ad_id
  WHERE ra.user_id = u.user_id
 ) stats ON true
`

const adFilter = `
 WHERE a.status='active'
 AND ($1::text='' OR strpos(lower(a.title),lower($1))>0
      OR strpos(lower(COALESCE(a.description,'')),lower($1))>0)
 AND ($2::integer=0 OR a.category_id=$2)
 AND ($3::text='' OR lower(a.city)=lower($3))
 AND ($4::bigint IS NULL OR a.price >= $4)
 AND ($5::bigint IS NULL OR a.price <= $5)
 AND ($6::boolean IS NULL OR a.has_delivery=$6)
`
const countAdsQuery = `SELECT count(*) FROM ad a` + adFilter

const listAdsQuery = `
 SELECT ` + adCardColumns + `
 FROM ad a
` + adMainImageJoin + adFilter + `
 ORDER BY
 CASE WHEN $7::text='price_asc' THEN a.price END ASC,
 CASE WHEN $7::text='price_desc' THEN a.price END DESC,
 CASE WHEN $7::text='newest' THEN a.created_at END DESC,
 a.id DESC
 LIMIT $8 OFFSET $9
`

const countMyAdsQuery = `
 SELECT status, count(*)
 FROM ad
 WHERE user_id=$1
 GROUP BY status
`

const listMyAdsQuery = `
 SELECT ` + adCardColumns + `
 FROM ad a
` + adMainImageJoin + `
 WHERE a.user_id=$1 AND ($2::text='' OR a.status=$2)
 ORDER BY a.updated_at DESC, a.id DESC
 LIMIT $3 OFFSET $4
`

const getAdForUpdateQuery = `
 SELECT user_id, status
 FROM ad
 WHERE id=$1
 FOR UPDATE
`

const closeAdQuery = `
 UPDATE ad
 SET status='closed', close_reason=$2
 WHERE id=$1
`

const deleteAdQuery = `DELETE FROM ad WHERE id=$1`

const getListingQuery = `
 SELECT ` + adCardColumns + `,
 c.category_id, c.name,
 u.user_id, u.first_name, COALESCE(u.avatar,''),
 COALESCE(stats.rating,0), COALESCE(stats.reviews_count,0), u.created_at,
 EXISTS (
  SELECT 1 FROM cart_item ci JOIN cart ct ON ct.id=ci.cart_id
  WHERE ct.user_id=$2 AND ci.ad_id=a.id
 ),
 (SELECT min(e.changed_at) FROM ad_status_event e WHERE e.ad_id=a.id AND e.status='active')
 FROM ad a
 JOIN user_account u ON u.user_id=a.user_id
 LEFT JOIN category c ON c.category_id=a.category_id
` + adMainImageJoin + sellerStatsJoin + `
 WHERE a.id=$1
`

const categoryPathQuery = `
 WITH RECURSIVE chain AS (
  SELECT category_id, parent_id, name, 0 AS depth
  FROM category WHERE category_id=$1
  UNION ALL
  SELECT c.category_id, c.parent_id, c.name, chain.depth+1
  FROM category c JOIN chain ON c.category_id=chain.parent_id
  WHERE chain.depth < 10
 )
 SELECT COALESCE(string_agg(name, ' › ' ORDER BY depth DESC), '') FROM chain
`

const listAdPhotosQuery = `
 SELECT image_id, url
 FROM ad_image
 WHERE ad_id=$1
 ORDER BY position, image_id
`
