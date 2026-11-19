-- migrate:up transaction:false

pragma auto_vacuum = incremental;
vacuum;

-- migrate:down transaction:false

pragma auto_vacuum = none;
vacuum;
