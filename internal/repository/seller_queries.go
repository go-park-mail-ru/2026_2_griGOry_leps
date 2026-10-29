package repository

const sellerStatsJoin = `
 LEFT JOIN LATERAL (
  SELECT ROUND(AVG(rv.rating), 2)::float8 AS rating, COUNT(*) AS reviews_count
  FROM review rv
  WHERE rv.seller_id = u.user_id
 ) stats ON true
`

const getSellerQuery = `
 SELECT u.user_id, u.first_name, COALESCE(u.avatar,''),
 COALESCE(stats.rating,0), COALESCE(stats.reviews_count,0), u.created_at,
 COALESCE(c.name,''), u.phone_verified,
 (SELECT count(*) FROM ad WHERE user_id=u.user_id AND status='sold'),
 (SELECT count(*) FROM ad WHERE user_id=u.user_id AND status='active')
 FROM user_account u
 LEFT JOIN city c ON c.city_id=u.city_id
` + sellerStatsJoin + `
 WHERE u.user_id=$1
`

const sellerExistsQuery = `SELECT EXISTS (SELECT 1 FROM user_account WHERE user_id=$1)`

const sellerAdFilter = `
 WHERE a.user_id=$1 AND a.status='active'
 AND ($2::integer=0 OR a.category_id=$2)
`

const countSellerAdsQuery = `SELECT count(*) FROM ad a` + sellerAdFilter

const listSellerAdsQuery = `
 SELECT ` + adCardColumns + `
 FROM ad a
` + adMainImageJoin + sellerAdFilter + `
 ORDER BY
 CASE WHEN $3::text='price_asc' THEN a.price END ASC,
 CASE WHEN $3::text='price_desc' THEN a.price END DESC,
 CASE WHEN $3::text='newest' THEN a.created_at END DESC,
 a.id DESC
 LIMIT $4 OFFSET $5
`

const listSellerCategoriesQuery = `
 SELECT c.category_id, c.name, count(*)
 FROM ad a
 JOIN category c ON c.category_id=a.category_id
 WHERE a.user_id=$1 AND a.status='active'
 GROUP BY c.category_id, c.name
 ORDER BY count(*) DESC, c.name
`

const sellerReviewFilter = `
 FROM review rv
 WHERE rv.seller_id=$1
 AND (NOT $2::boolean OR EXISTS (SELECT 1 FROM review_image ri WHERE ri.rew_id=rv.rew_id))
`

const countSellerReviewsQuery = `SELECT count(*)` + sellerReviewFilter

const listSellerReviewsQuery = `
 SELECT rv.rew_id, au.user_id, au.first_name, COALESCE(au.avatar,''),
 rv.rating::float8, COALESCE(rv.text,''), COALESCE(rv.listing_title,''), rv.created_at,
 COALESCE((SELECT array_agg(ri.url ORDER BY ri.position, ri.image_id)
           FROM review_image ri WHERE ri.rew_id=rv.rew_id), '{}'::text[])
 FROM review rv
 JOIN user_account au ON au.user_id=rv.user_id
 WHERE rv.seller_id=$1
 AND (NOT $2::boolean OR EXISTS (SELECT 1 FROM review_image ri WHERE ri.rew_id=rv.rew_id))
 ORDER BY
 CASE WHEN $3::text='rating_desc' THEN rv.rating END DESC,
 CASE WHEN $3::text='rating_asc' THEN rv.rating END ASC,
 rv.created_at DESC,
 rv.rew_id DESC
 LIMIT $4 OFFSET $5
`

const sellerRatingQuery = `
 SELECT COALESCE(ROUND(AVG(rv.rating), 2)::float8, 0)
 FROM review rv
 WHERE rv.seller_id=$1
`

const sellerRatingDistributionQuery = `
 SELECT ROUND(rv.rating)::int AS stars, count(*)
 FROM review rv
 WHERE rv.seller_id=$1
 GROUP BY stars
`

const createSellerReportQuery = `
 INSERT INTO seller_report (seller_id, reporter_id, reason, comment)
 VALUES ($1, $2, $3, $4)
`
