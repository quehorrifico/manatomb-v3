-- ONLY for the disposable manatomb_v2_local database; synthetic catalog IDs.
DO $$ BEGIN
 IF current_database() <> 'manatomb_v2_local' THEN RAISE EXCEPTION 'Local fixture database required'; END IF;
 IF NOT EXISTS (SELECT 1 FROM users WHERE id=1 AND email='v2-local@example.invalid') THEN RAISE EXCEPTION 'Local fixture owner required'; END IF;
END $$;
INSERT INTO oracle_cards(oracle_id,name,type_line,mana_cost,color_identity,colors,commander_legal,is_commander_candidate,power_text,toughness_text,layout)
VALUES ('00000000-0000-4000-8000-000000000101','Silvos, Rogue Elemental','Legendary Creature — Elemental','{3}{G}{G}{G}',ARRAY['G'],ARRAY['G'],true,true,'8','5','normal'),
       ('00000000-0000-4000-8000-000000000102','Forest','Basic Land — Forest','',ARRAY['G'],ARRAY[]::text[],true,false,null,null,'normal'),
       ('00000000-0000-4000-8000-000000000103','Plains','Basic Land — Plains','',ARRAY['W'],ARRAY[]::text[],true,false,null,null,'normal')
ON CONFLICT(oracle_id) DO NOTHING;
INSERT INTO decks(user_id,name,description,format,commander_name)
SELECT 1,'V2 local recovery fixture','Original saved notes','Commander','Silvos, Rogue Elemental'
WHERE NOT EXISTS(SELECT 1 FROM decks WHERE user_id=1 AND name='V2 local recovery fixture');
INSERT INTO deck_cards(deck_id,oracle_id,qty,board)
SELECT d.id,v.id::uuid,v.qty,v.board FROM decks d CROSS JOIN (VALUES
 ('00000000-0000-4000-8000-000000000101',1,'main'),
 ('00000000-0000-4000-8000-000000000102',99,'main'),
 ('00000000-0000-4000-8000-000000000103',2,'side'),
 ('00000000-0000-4000-8000-000000000103',3,'maybe')) AS v(id,qty,board)
WHERE d.user_id=1 AND d.name='V2 local recovery fixture'
ON CONFLICT(deck_id,oracle_id,board) DO NOTHING;
SELECT id,name FROM decks WHERE user_id=1 AND name='V2 local recovery fixture';
