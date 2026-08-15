// Enum values the API stores are machine identifiers. The panel is written in
// Russian, so translate them here rather than letting "basic" or "active" reach
// the interface. Unknown values fall through unchanged: a new state should look
// odd in review, not disappear.

const CUSTOMER_LEVEL: Record<string, string> = {
  basic: "Базовый",
  silver: "Серебряный",
  gold: "Золотой",
  vip: "VIP",
};

const SUBSCRIPTION_STATUS: Record<string, string> = {
  trial: "Пробный период",
  active: "Активна",
  past_due: "Просрочена",
  cancelled: "Отменена",
  canceled: "Отменена",
  expired: "Истекла",
  suspended: "Приостановлена",
};

const WORKSPACE_ROLE: Record<string, string> = {
  owner: "Владелец",
  admin: "Администратор",
  manager: "Управляющий",
  staff: "Сотрудник",
  company_owner: "Владелец",
  employee: "Сотрудник",
  super_admin: "Администратор платформы",
};

function translate(dictionary: Record<string, string>, value: string | undefined | null) {
  if (!value) return "";
  return dictionary[value.toLowerCase()] ?? value;
}

export const customerLevelLabel = (value: string | undefined | null) => translate(CUSTOMER_LEVEL, value);
export const subscriptionStatusLabel = (value: string | undefined | null) => translate(SUBSCRIPTION_STATUS, value);
export const workspaceRoleLabel = (value: string | undefined | null) => translate(WORKSPACE_ROLE, value);

// Company and account states shown in the platform console. Kept apart because
// Russian agreement differs: компания активна, пользователь активен.
const COMPANY_STATUS: Record<string, string> = {
  active: "Активна",
  archived: "Скрыта",
  blocked: "Заблокирована",
  suspended: "Приостановлена",
  cancelled: "Отменена",
};

const ACCOUNT_STATUS: Record<string, string> = {
  active: "Активен",
  archived: "Скрыт",
  blocked: "Заблокирован",
  invited: "Приглашён",
  disabled: "Отключён",
};

export const companyStatusLabel = (value: string | undefined | null) => translate(COMPANY_STATUS, value);
export const accountStatusLabel = (value: string | undefined | null) => translate(ACCOUNT_STATUS, value);

const DELIVERY_STATUS: Record<string, string> = {
  sent: "Отправлено",
  failed: "Ошибка",
  queued: "В очереди",
  delivered: "Доставлено",
};

const FILE_KIND: Record<string, string> = {
  logo: "Логотип",
  asset: "Изображение",
  document: "Документ",
};

export const deliveryStatusLabel = (value: string | undefined | null) => translate(DELIVERY_STATUS, value);
export const fileKindLabel = (value: string | undefined | null) => translate(FILE_KIND, value);
