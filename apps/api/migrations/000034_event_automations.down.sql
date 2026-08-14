DELETE FROM campaign_automations WHERE trigger_type IN ('near_reward','reward_unlocked','nfc_registration');
ALTER TABLE campaign_automations DROP CONSTRAINT IF EXISTS campaign_automations_trigger_type_check;
ALTER TABLE campaign_automations ADD CONSTRAINT campaign_automations_trigger_type_check
  CHECK(trigger_type IN ('birthday_bonus','bonus_expiry_3d','winback_30d'));
