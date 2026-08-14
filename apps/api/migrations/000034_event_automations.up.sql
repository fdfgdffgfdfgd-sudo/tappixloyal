ALTER TABLE campaign_automations DROP CONSTRAINT IF EXISTS campaign_automations_trigger_type_check;
ALTER TABLE campaign_automations ADD CONSTRAINT campaign_automations_trigger_type_check
  CHECK(trigger_type IN ('birthday_bonus','bonus_expiry_3d','winback_30d','near_reward','reward_unlocked','nfc_registration'));

INSERT INTO campaign_automations(company_id,trigger_type,name,subject,message,settings,is_active)
SELECT id,'near_reward','До подарка остался один визит','До подарка остался один визит','{{name}}, вам осталось всего одно посещение до подарка. Ждём вас снова!', '{}'::jsonb, true FROM companies
ON CONFLICT(company_id,trigger_type) DO NOTHING;

INSERT INTO campaign_automations(company_id,trigger_type,name,subject,message,settings,is_active)
SELECT id,'reward_unlocked','Награда получена','Ваша награда уже доступна','{{name}}, награда уже доступна. Покажите карту сотруднику при следующем визите.', '{}'::jsonb, true FROM companies
ON CONFLICT(company_id,trigger_type) DO NOTHING;

INSERT INTO campaign_automations(company_id,trigger_type,name,subject,message,settings,is_active)
SELECT id,'nfc_registration','Новый клиент после NFC-регистрации','Добро пожаловать в программу','{{name}}, ваша цифровая карта готова. Откройте её перед следующим визитом.', '{}'::jsonb, true FROM companies
ON CONFLICT(company_id,trigger_type) DO NOTHING;
