CREATE UNIQUE INDEX IF NOT EXISTS bookings_active_slot_uidx
  ON bookings(branch_id, starts_at)
  WHERE status IN ('new','confirmed');
