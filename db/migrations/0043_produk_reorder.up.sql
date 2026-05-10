-- Reorder point fields untuk inventory forecasting.
-- ROP = (avg_daily_sales * lead_time_days) + safety_stock
-- avg_daily_sales dihitung on-demand dari penjualan_item history.
ALTER TABLE produk ADD COLUMN IF NOT EXISTS lead_time_days INT NOT NULL DEFAULT 7;
ALTER TABLE produk ADD COLUMN IF NOT EXISTS safety_stock NUMERIC(14,4) NOT NULL DEFAULT 0;
