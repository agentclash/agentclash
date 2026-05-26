-- +goose Up
ALTER TABLE runtime_profiles DROP CONSTRAINT IF EXISTS runtime_profiles_execution_target_check;
ALTER TABLE runtime_profiles ADD CONSTRAINT runtime_profiles_execution_target_check
    CHECK (execution_target IN ('native', 'hosted_external', 'prompt_eval', 'responses', 'multi_turn'));

-- +goose Down
ALTER TABLE runtime_profiles DROP CONSTRAINT IF EXISTS runtime_profiles_execution_target_check;
ALTER TABLE runtime_profiles ADD CONSTRAINT runtime_profiles_execution_target_check
    CHECK (execution_target IN ('native', 'hosted_external'));
