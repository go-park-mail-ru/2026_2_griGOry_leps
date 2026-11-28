package repository

const createConf = `
 WITH created AS (
    INSERT INTO onlineconf_entries (scope,name,kind,value) VALUES ($1,$2,$3,$4::jsonb)
    RETURNING *
 ), logged AS (
    INSERT INTO onlineconf_entry_history (entry_id,actor_id,operation,after_data)
    SELECT id,$5,'create',to_jsonb(created) FROM created
 )
 SELECT id,scope,name,kind,value FROM created`

const updateConf = `
 WITH previous AS (
    SELECT * FROM onlineconf_entries WHERE id=$1 FOR UPDATE
 ), updated AS (
    UPDATE onlineconf_entries e
    SET scope=COALESCE($2,p.scope),name=COALESCE($3,p.name),value=COALESCE($4::jsonb,p.value)
    FROM previous p WHERE e.id=p.id RETURNING e.*
 ), logged AS (
    INSERT INTO onlineconf_entry_history (entry_id,actor_id,operation,before_data,after_data)
    SELECT u.id,$5,'update',to_jsonb(p),to_jsonb(u)
    FROM updated u JOIN previous p USING (id)
 )
 SELECT id,scope,name,kind,value FROM updated`

const deleteConf = `
 WITH previous AS (
    SELECT * FROM onlineconf_entries WHERE id=$1 FOR UPDATE
 ), deleted AS (
    DELETE FROM onlineconf_entries e USING previous p WHERE e.id=p.id RETURNING e.id
 ), logged AS (
    INSERT INTO onlineconf_entry_history (entry_id,actor_id,operation,before_data)
    SELECT p.id,$2,'delete',to_jsonb(p) FROM previous p JOIN deleted USING (id)
 )
 SELECT id FROM deleted`

const historyConf = `
 SELECT COALESCE((
    SELECT jsonb_agg(jsonb_build_object(
        'id',id,'entry_id',entry_id,'actor_id',actor_id,'operation',operation,
        'changed_at',changed_at,'before',before_data,'after',after_data
    ) ORDER BY id DESC)
    FROM onlineconf_entry_history WHERE entry_id=e.id
 ), '[]'::jsonb)
 FROM onlineconf_entries e WHERE e.id=$1`

const searchConf = `
		SELECT id, scope, name, kind, value
		FROM onlineconf_entries
		WHERE strpos(lower(name), lower($1)) > 0
		  AND ($2::text IS NULL OR scope = $2)
		  AND ($3::text IS NULL OR kind = $3)
		ORDER BY id`
