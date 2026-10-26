package repository

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
 SELECT a.id,a.user_id,COALESCE(a.category_id,0),a.title,
 COALESCE(a.description,''),a.price,
 COALESCE(a.city,''),a.has_delivery,COALESCE(image.url,''),a.status,a.created_at
 FROM ad a
 LEFT JOIN LATERAL (
  SELECT url FROM ad_image WHERE ad_id=a.id ORDER BY position,image_id LIMIT 1
 ) image ON true
` + adFilter + `
 ORDER BY
 CASE WHEN $7::text='price_asc' THEN a.price END ASC,
 CASE WHEN $7::text='price_desc' THEN a.price END DESC,
 CASE WHEN $7::text='newest' THEN a.created_at END DESC,
 a.id DESC
 LIMIT $8 OFFSET $9
`
