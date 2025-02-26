CREATE NAMESPACE IF NOT EXISTS primitive_stream_db_name;

{primitive_stream_db_name} CREATE TABLE primitive_events (
                                  date_value INT NOT NULL,          -- unix timestamp
                                  value DECIMAL(36,18) NOT NULL,
                                  created_at INT NOT NULL,          -- based on blockheight
                                  PRIMARY KEY (date_value, created_at)
);

{primitive_stream_db_name} CREATE TABLE metadata (
                          row_id UUID PRIMARY KEY NOT NULL,
                          metadata_key TEXT NOT NULL,
                          value_i INT,
                          value_f DECIMAL(36,18),
                          value_b BOOL,
                          value_s TEXT,
                          value_ref TEXT,
                          created_at INT NOT NULL,          -- block height
                          disabled_at INT
);

-- TODO: it is not written in the kwil docs, but I am able to pass the `exec-sql` command
-- might worth to double check if this is the correct way to do it or whether we can use indexes
CREATE INDEX key_idx ON metadata(metadata_key);
CREATE INDEX ref_idx ON metadata(value_ref);
CREATE INDEX created_idx ON metadata(created_at);

---------------------------------------------------------------
-- ACTION definitions
---------------------------------------------------------------

{primitive_stream_db_name} CREATE ACTION is_initiated() PRIVATE VIEW RETURNS (result BOOL) {
    for $row in SELECT * FROM metadata WHERE metadata_key = 'type' LIMIT 1 {
        return true;
    }
    return false;
};

{primitive_stream_db_name} CREATE ACTION is_stream_owner($wallet TEXT) PUBLIC VIEW RETURNS (result BOOL) {
    for $row in SELECT * FROM metadata
                WHERE metadata_key = 'stream_owner'
                  AND value_ref = LOWER($wallet)
                    LIMIT 1 {
        return true;
    }
    return false;
};

{primitive_stream_db_name} CREATE ACTION is_wallet_allowed_to_write($wallet TEXT) PUBLIC VIEW RETURNS (value BOOL) {
    if is_stream_owner($wallet) {
        return true;
    }
    for $row in get_metadata('allow_write_wallet', false, $wallet) {
        return true;
    }
    return false;
};

{primitive_stream_db_name} CREATE ACTION is_wallet_allowed_to_read($wallet TEXT) PUBLIC VIEW RETURNS (value BOOL) {
    $visibility INT := 0;
    for $v_row in get_metadata('read_visibility', true, null) {
        $visibility := $v_row.value_i;
    }
    if $visibility == 0 {
        return true;
    }
    if is_stream_owner($wallet) {
        return true;
    }
    for $row in get_metadata('allow_read_wallet', false, $wallet) {
        return true;
    }
    return false;
};

{primitive_stream_db_name} CREATE ACTION stream_owner_only() PRIVATE VIEW {
    if is_stream_owner(@caller) == false {
        error('Stream owner only procedure');
    }
};

{primitive_stream_db_name} CREATE ACTION init() PUBLIC OWNER {
    if is_initiated() {
        error('this contract was already initialized');
    }
    if @caller == '' {
        error('caller is empty');
    }
    $current_block INT := @height;
    $current_uuid UUID := uuid_generate_v5('111bfa42-17a2-11ef-bf03-325096b39f47'::uuid, @txid);
    $current_uuid := uuid_generate_v5($current_uuid, @txid);
    INSERT INTO metadata (row_id, metadata_key, value_s, created_at)
    VALUES ($current_uuid, 'type', 'primitive', $current_block);
    $current_uuid := uuid_generate_v5($current_uuid, @txid);
    INSERT INTO metadata (row_id, metadata_key, value_ref, created_at)
    VALUES ($current_uuid, 'stream_owner', LOWER(@caller), 1);
    $current_uuid := uuid_generate_v5($current_uuid, @txid);
    INSERT INTO metadata (row_id, metadata_key, value_i, created_at)
    VALUES ($current_uuid, 'compose_visibility', 0, $current_block);
    $current_uuid := uuid_generate_v5($current_uuid, @txid);
    INSERT INTO metadata (row_id, metadata_key, value_i, created_at)
    VALUES ($current_uuid, 'read_visibility', 0, $current_block);
    $readonly_keys TEXT[] := [
        'type',
        'stream_owner',
        'readonly_key'
    ];
    for $key in $readonly_keys {
        $current_uuid := uuid_generate_v5($current_uuid, @txid);
    INSERT INTO metadata (row_id, metadata_key, value_s, created_at)
    VALUES ($current_uuid, 'readonly_key', $key, $current_block);
    }
};

{primitive_stream_db_name} CREATE ACTION insert_metadata($key TEXT, $value TEXT, $val_type TEXT) PUBLIC {
    $value_i INT;
    $value_s TEXT;
    $value_f DECIMAL(36,18);
    $value_b BOOL;
    $value_ref TEXT;

    if $val_type == 'int' {
        $value_i := $value::int;
    } elseif $val_type == 'string' {
        $value_s := $value;
    } elseif $val_type == 'bool' {
        $value_b := $value::bool;
    } elseif $val_type == 'ref' {
        $value_ref := $value;
    } elseif $val_type == 'float' {
        $value_f := $value::decimal(36,18);
    } else {
        error(format('unknown type used "%s". valid types = "float" | "bool" | "int" | "ref" | "string"', $val_type));
    }

    stream_owner_only();

    if is_initiated() == false {
        error('contract must be initiated');
    }

    for $row in SELECT * FROM metadata WHERE metadata_key = 'readonly_key' AND value_s = $key LIMIT 1 {
        error('Cannot insert metadata for read-only key');
    }

    $uuid_key := @txid || $key || $value;
    $uuid UUID := uuid_generate_v5('1361df5d-0230-47b3-b2c1-37950cf51fe9'::uuid, $uuid_key);
    $current_block INT := @height;

    INSERT INTO metadata (row_id, metadata_key, value_i, value_f, value_s, value_b, value_ref, created_at)
    VALUES ($uuid, $key, $value_i, $value_f, $value_s, $value_b, LOWER($value_ref), $current_block);
};

{primitive_stream_db_name} CREATE ACTION get_metadata($key TEXT, $only_latest BOOL, $ref TEXT) PUBLIC VIEW RETURNS (
    row_id UUID,
    value_i INT,
    value_f DECIMAL(36,18),
    value_b BOOL,
    value_s TEXT,
    value_ref TEXT,
    created_at INT
) {
    if $only_latest == true {
        if $ref is distinct from null {
            return SELECT
                              row_id,
                              null::int as value_i,
                                  null::decimal(36,18) as value_f,
                                  null::bool as value_b,
                                  null::text as value_s,
                                  value_ref,
                              created_at
                   FROM metadata
                   WHERE metadata_key = $key AND disabled_at IS NULL AND value_ref = LOWER($ref)
                   ORDER BY created_at DESC
                              LIMIT 1;
        } else {
            return SELECT
                       row_id,
                       value_i,
                       value_f,
                       value_b,
                       value_s,
                       value_ref,
                       created_at
                   FROM metadata
                   WHERE metadata_key = $key AND disabled_at IS NULL
                   ORDER BY created_at DESC
                       LIMIT 1;
        }
        } else {
        if $ref is distinct from null {
            return SELECT
                       row_id,
                       null::int as value_i,
                           null::decimal(36,18) as value_f,
                           null::bool as value_b,
                           null::text as value_s,
                           value_ref,
                       created_at
                   FROM metadata
                   WHERE metadata_key = $key AND disabled_at IS NULL AND value_ref = LOWER($ref)
                   ORDER BY created_at DESC;
        } else {
            return SELECT
                       row_id,
                       value_i,
                       value_f,
                       value_b,
                       value_s,
                       value_ref,
                       created_at
                   FROM metadata
                   WHERE metadata_key = $key AND disabled_at IS NULL
                   ORDER BY created_at DESC;
        }
    }
};

{primitive_stream_db_name} CREATE ACTION disable_metadata($row_id UUID) PUBLIC {
    stream_owner_only();
    $current_block INT := @height;
    $found BOOL := false;
    for $metadata_row in
    SELECT metadata_key
    FROM metadata
    WHERE row_id = $row_id AND disabled_at IS NULL
    LIMIT 1 {
        $found := true;
        $row_key TEXT := $metadata_row.metadata_key;
        for $readonly_row in SELECT row_id FROM metadata WHERE metadata_key = 'readonly_key' AND value_s = $row_key LIMIT 1 {
            error('Cannot disable read-only metadata');
        }
        UPDATE metadata SET disabled_at = $current_block
        WHERE row_id = $row_id;
    }

    if $found == false {
        error('metadata record not found');
    }
};

{primitive_stream_db_name} CREATE ACTION insert_record($date_value INT, $value DECIMAL(36,18)) PUBLIC {
    if is_wallet_allowed_to_write(@caller) == false {
        error('wallet not allowed to write');
    }
    if is_initiated() == false {
        error('contract must be initiated');
    }
    $current_block INT := @height;
    INSERT INTO primitive_events (date_value, value, created_at)
    VALUES ($date_value, $value, $current_block);
};

-- TODO: This action is commented out because it will cause error when combining the calling of another action and sql query
-- see get_metadata action with order by and limit 1
-- {primitive_stream_db_name} CREATE ACTION get_index($date_from INT, $date_to INT, $frozen_at INT, $base_date INT) PUBLIC VIEW RETURNS (
--     date_value INT,
--     value DECIMAL(36,18)
-- ) {
--     $effective_base_date INT := $base_date;
--     if ($effective_base_date == 0 OR $effective_base_date IS NULL) {
--         for $v_row in get_metadata('default_base_date', true, null) ORDER BY created_at DESC LIMIT 1 {
--             $effective_base_date := $v_row.value_i;
--         }
--     }
--     $baseValue DECIMAL(36,18) := get_base_value($effective_base_date, $frozen_at);
--     if $baseValue == 0::decimal(36,18) {
--         error('base value is 0');
--     }
--     return SELECT date_value, (value * 100::decimal(36,18)) / $baseValue as value FROM get_record($date_from, $date_to, $frozen_at);
-- };

{primitive_stream_db_name} CREATE ACTION get_base_value($base_date INT, $frozen_at INT) PRIVATE VIEW RETURNS (value DECIMAL(36,18)) {
    if $base_date is null OR $base_date = 0 {
        for $row in SELECT * FROM primitive_events
        WHERE (created_at <= $frozen_at OR $frozen_at = 0 OR $frozen_at IS NULL)
        ORDER BY date_value ASC, created_at DESC LIMIT 1 {
            return $row.value;
        }
    }
    for $row2 in SELECT * FROM primitive_events
        WHERE date_value <= $base_date AND (created_at <= $frozen_at OR $frozen_at = 0 OR $frozen_at IS NULL)
        ORDER BY date_value DESC, created_at DESC LIMIT 1 {
            return $row2.value;
    }
    for $row3 in SELECT * FROM primitive_events
        WHERE date_value > $base_date AND (created_at <= $frozen_at OR $frozen_at = 0 OR $frozen_at IS NULL)
        ORDER BY date_value ASC, created_at DESC LIMIT 1 {
            return $row3.value;
    }
    error('no base value found');
};

{primitive_stream_db_name} CREATE ACTION get_first_record($after_date INT, $frozen_at INT) PUBLIC VIEW RETURNS (
    date_value INT,
    value DECIMAL(36,18)
) {
    if is_wallet_allowed_to_read(@caller) == false {
        error('wallet not allowed to read');
    }
    is_stream_allowed_to_compose(@foreign_caller);
    if $after_date is null {
        $after_date := 0;
    }
    if $frozen_at is null {
        $frozen_at := 0;
    }
    return SELECT date_value, value FROM primitive_events
           WHERE date_value >= $after_date
             AND (created_at <= $frozen_at OR $frozen_at = 0 OR $frozen_at IS NULL)
           ORDER BY date_value ASC, created_at DESC LIMIT 1;
};

{primitive_stream_db_name} CREATE ACTION get_original_record($date_from INT, $date_to INT, $frozen_at INT) PRIVATE VIEW RETURNS (
    date_value INT,
    value DECIMAL(36,18)
) {
    if is_wallet_allowed_to_read(@caller) == false {
        error('wallet not allowed to read');
    }
    is_stream_allowed_to_compose(@foreign_caller);
    $frozenValue INT := 0;
    if $frozen_at IS DISTINCT FROM NULL {
        $frozenValue := $frozen_at::int;
    }
    $last_result_date INT := 0;
    if $date_from IS DISTINCT FROM NULL {
        if $date_to IS DISTINCT FROM NULL {
            for $row in SELECT date_value, value FROM primitive_events
                        WHERE date_value >= $date_from AND date_value <= $date_to
                          AND (created_at <= $frozenValue OR $frozenValue = 0)
                        ORDER BY date_value DESC, created_at DESC {
                if $last_result_date != $row.date_value {
                    $last_result_date := $row.date_value;
                    return next $row.date_value, $row.value;
                }
            }
        } else {
            for $row2 in SELECT date_value, value FROM primitive_events
                         WHERE date_value >= $date_from
                           AND (created_at <= $frozenValue OR $frozenValue = 0)
                         ORDER BY date_value DESC, created_at DESC {
                if $last_result_date != $row2.date_value {
                    $last_result_date := $row2.date_value;
                    return next $row2.date_value, $row2.value;
                }
            }
        }
    } else {
        if $date_to IS NOT DISTINCT FROM NULL {
            return SELECT date_value, value FROM primitive_events
                   WHERE created_at <= $frozenValue OR $frozenValue = 0
                       AND $last_result_date != date_value
                   ORDER BY date_value DESC, created_at DESC LIMIT 1;
        } else {
            error('date_from is required if date_to is provided');
        }
    }
};

{primitive_stream_db_name} CREATE ACTION get_record($date_from INT, $date_to INT, $frozen_at INT) PUBLIC VIEW RETURNS (
    date_value INT,
    value DECIMAL(36,18)
) {
    $is_first_result BOOL := true;
    for $row in get_original_record($date_from, $date_to, $frozen_at) {
        if $is_first_result == true {
            $first_result_date INT := $row.date_value;
            if $first_result_date != $date_from {
                for $last_row in get_last_record_before_date($first_result_date) {
                    return next $last_row.date_value, $last_row.value;
                }
            }
            $is_first_result := false;
        }
        return next $row.date_value, $row.value;
    }
    if $is_first_result == true {
        for $last_row2 in get_last_record_before_date($date_from) {
            return next $last_row2.date_value, $last_row2.value;
        }
    }
};

{primitive_stream_db_name} CREATE ACTION get_last_record_before_date($date_from INT) PUBLIC VIEW RETURNS (
    date_value INT,
    value DECIMAL(36,18)
) {
    return SELECT date_value, value FROM primitive_events
           WHERE date_value < $date_from
           ORDER BY date_value DESC, created_at DESC LIMIT 1;
};

{primitive_stream_db_name} CREATE ACTION transfer_stream_ownership($new_owner TEXT) PUBLIC {
    stream_owner_only();
    check_eth_address($new_owner);
    UPDATE metadata SET value_ref = LOWER($new_owner)
    WHERE metadata_key = 'stream_owner';
};

{primitive_stream_db_name} CREATE ACTION check_eth_address($address TEXT) PRIVATE {
    if (length($address) != 42) {
        error('invalid address length');
    }
    for $row in SELECT $address LIKE '0x%' as a {
        if $row.a == false {
            error('address does not start with 0x');
        }
    }
};

-- TODO: This action is commented out because it will cause error when combining the calling of another action and sql query
-- see get_metadata action with limit 1
-- {primitive_stream_db_name} CREATE ACTION is_stream_allowed_to_compose($foreign_caller TEXT) PUBLIC VIEW RETURNS (value BOOL) {
--     if $foreign_caller == '' {
--         return true;
-- }
--     $visibility INT := 0;
-- for $v_row in get_metadata('compose_visibility', true, null) {
--         $visibility := $v_row.value_i;
-- }
--     if $visibility == 0 {
--         return true;
-- }
--     for $row in get_metadata('allow_compose_stream', true, $foreign_caller) LIMIT 1 {
--         return true;
-- }
--     error('stream not allowed to compose');
-- };

{primitive_stream_db_name} CREATE ACTION get_index_change($date_from INT, $date_to INT, $frozen_at INT, $base_date INT, $days_interval INT) PUBLIC VIEW RETURNS (
    date_value INT,
    value DECIMAL(36,18)
) {
    if $frozen_at == null {
        $frozen_at := 0;
    }
    if $days_interval == null {
        error('days_interval is required');
    }
    $current_values DECIMAL(36,18)[];
    $current_dates INT[];
    $expected_prev_dates INT[];
    for $row_current in get_index($date_from, $date_to, $frozen_at, $base_date) {
        $prev_date := $row_current.date_value - ($days_interval * 86400);
        $expected_prev_dates := array_append($expected_prev_dates, $prev_date);
        $current_values := array_append($current_values, $row_current.value);
        $current_dates := array_append($current_dates, $row_current.date_value);
    }
    $earliest_prev_date := $expected_prev_dates[1];
    $latest_prev_date := $expected_prev_dates[array_length($expected_prev_dates)];
    $real_prev_values DECIMAL(36,18)[];
    $real_prev_dates INT[];
    for $row_prev in get_index($earliest_prev_date, $latest_prev_date, $frozen_at, $base_date) {
        $real_prev_values := array_append($real_prev_values, $row_prev.value);
        $real_prev_dates := array_append($real_prev_dates, $row_prev.date_value);
    }
    $result_prev_dates INT[];
    $result_prev_values DECIMAL(36,18)[];
    $real_prev_date_idx INT := 1;
    if array_length($expected_prev_dates) > 0 {
        for $expected_prev_date_idx in 1..array_length($expected_prev_dates) {
            for $selector in $real_prev_date_idx..array_length($real_prev_dates) {
                if $real_prev_dates[$selector + 1] > $expected_prev_dates[$expected_prev_date_idx]
                   OR $real_prev_dates[$selector + 1] IS NULL {
                    if $real_prev_dates[$selector] > $expected_prev_dates[$expected_prev_date_idx] {
                        $result_prev_dates := array_append($result_prev_dates, null::int);
                        $result_prev_values := array_append($result_prev_values, null::decimal(36,18));
                    } else {
                        $result_prev_dates := array_append($result_prev_dates, $real_prev_dates[$selector]);
                        $result_prev_values := array_append($result_prev_values, $real_prev_values[$selector]);
                    }
                    $real_prev_date_idx := $selector;
                    break;
                }
            }
        }
    }
    if array_length($current_dates) != array_length($result_prev_dates) {
        error('we have different number of dates and values');
    }
    if array_length($current_values) != array_length($result_prev_values) {
        error('we have different number of dates and values');
    }
    if array_length($result_prev_dates) > 0 {
        for $row_result in 1..array_length($result_prev_dates) {
            if $result_prev_dates[$row_result] IS DISTINCT FROM NULL {
                return next $current_dates[$row_result],
                    (($current_values[$row_result] - $result_prev_values[$row_result]) * 100.00::decimal(36,18))
                    / $result_prev_values[$row_result];
            }
        }
    }
};
