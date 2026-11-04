-- ad_publications records every ad published through POST /ads, to enforce the publication quota: a user may
-- publish at most 5 ads over a rolling 24 hours.
--
-- It is a journal of publications, not a view of the ads: imported ads are not recorded, and deleting an ad does not
-- remove its publication, so that deleting and publishing again cannot get around the quota. Hence no foreign key
-- to ads.
CREATE TABLE ad_publications (
    user_id      UUID        NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Supports counting the recent publications of a user, and purging the publications past the quota window.
CREATE INDEX ad_publications_user_id_published_at_idx ON ad_publications (user_id, published_at);
