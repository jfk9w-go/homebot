create or replace view debit
            (id, authorization_id, time, debiting_time, type, "group", status, description, currency, amount,
             account_currency, account_amount, cashback_currency, cashback_amount, category, card_number, mcc,
             card_present, merchant_name, merchant_country, merchant_city, merchant_address, merchant_zip, account_id)
as
select o.id,
       o.authorization_id,
       o."time",
       o.debiting_time,
       o.type,
       o."group",
       o.status,
       o.description,
       o.currency,
       o.amount,
       o.account_currency,
       o.account_amount,
       o.cashback_currency,
       o.cashback_amount,
       o.category,
       o.card_number,
       o.mcc,
       o.card_present,
       o.merchant_name,
       o.merchant_country,
       o.merchant_city,
       o.merchant_address,
       o.merchant_zip,
       o.account_id
from operations o
where o.type = 'Debit'
  and o.status != 'FAILED'
  and o.account_currency = 'RUB'
  and o.category not in ('Наличные', 'Переводы')
  and o.description != 'Перевод между счетами'
order by o."time" desc;
