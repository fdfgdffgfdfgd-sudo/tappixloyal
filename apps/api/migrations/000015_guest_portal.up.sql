ALTER TABLE customers ADD COLUMN IF NOT EXISTS city varchar(120);

INSERT INTO company_settings(company_id, branding)
SELECT id, jsonb_build_object(
  'guestPortal', jsonb_build_object(
    'welcomeTitle', 'Добро пожаловать в ' || name || '!',
    'welcomeText', 'Получите бонусную карту бесплатно. Копите бонусы за каждое посещение и получайте персональные предложения.',
    'primaryColor', '#5B4AE8',
    'backgroundUrl', '',
    'logoUrl', coalesce(logo_url, ''),
    'requireEmail', false,
    'requireCity', false,
    'showGender', true,
    'promotionsEnabled', true,
    'promotionTitle', 'Специальное предложение',
    'promotionText', 'Покажите бонусную карту администратору и узнайте о доступных предложениях.',
    'referralBonus', 100,
    'whatsapp', '',
    'instagram', '',
    'website', '',
    'mapUrl', ''
  )
)
FROM companies
ON CONFLICT(company_id) DO UPDATE SET
  branding = company_settings.branding || jsonb_build_object(
    'guestPortal', coalesce(company_settings.branding->'guestPortal', excluded.branding->'guestPortal')
  );
