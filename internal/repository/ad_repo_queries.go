package repository

const adCardColumns = `
 a.id,a.user_id,COALESCE(a.category_id,0),a.title,
 COALESCE(a.description,''),a.price,
 COALESCE(a.city,''),a.has_delivery,a.is_free,COALESCE(image.url,''),a.status,
 COALESCE(a.close_reason,''),COALESCE(a.published_at,a.created_at),a.updated_at,COALESCE(a.rejection_reason,'')
`

const adMainImageJoin = `
 LEFT JOIN LATERAL (
  SELECT url FROM ad_image WHERE ad_id=a.id ORDER BY position,image_id LIMIT 1
 ) image ON true
`

const categoryBranch = `
 WITH RECURSIVE category_branch(category_id) AS (
  SELECT category_id FROM category WHERE category_id=$2
  UNION
  SELECT child.category_id
  FROM category child JOIN category_branch parent ON child.parent_id=parent.category_id
 )
`

const adFilter = `
 WHERE a.status='active'
 AND ($1::text='' OR strpos(lower(a.title),lower($1))>0
      OR strpos(lower(COALESCE(a.description,'')),lower($1))>0)
 AND ($2::integer=0 OR a.category_id IN (SELECT category_id FROM category_branch))
 AND ($3::text='' OR lower(a.city)=lower($3))
 AND ($4::bigint IS NULL OR a.price >= $4)
 AND ($5::bigint IS NULL OR a.price <= $5)
 AND ($6::boolean IS NULL OR a.has_delivery=$6)
`
const countAdsQuery = categoryBranch + `SELECT count(*) FROM ad a` + adFilter

const listAdsQuery = categoryBranch + `
 SELECT ` + adCardColumns + `
 FROM ad a
` + adMainImageJoin + adFilter + `
 AND ($7::timestamptz IS NULL OR (COALESCE(a.published_at,a.created_at),a.id) < ($7::timestamptz,$8::integer))
 ORDER BY
 CASE WHEN $9::text='price_asc' THEN a.price END ASC,
 CASE WHEN $9::text='price_desc' THEN a.price END DESC,
 CASE WHEN $9::text='newest' THEN COALESCE(a.published_at,a.created_at) END DESC,
 a.id DESC
 LIMIT $10 OFFSET $11
`

const countMyAdsQuery = `
 SELECT status, count(*)
 FROM ad
 WHERE user_id=$1
 GROUP BY status
`

const listMyAdsQuery = `
 SELECT ` + adCardColumns + `, (SELECT COUNT(*) FROM views WHERE ad_id = a.id AND is_first)
 FROM ad a
` + adMainImageJoin + `
 WHERE a.user_id=$1 AND ($2::text='' OR a.status=$2)
 ORDER BY a.updated_at DESC, a.id DESC
 LIMIT $3 OFFSET $4
`
