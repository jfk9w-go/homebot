create or replace view trading_positions
            (ticker, currency, buy_time, buy_price, quantity, sell_time, sell_price, username) as
with events_cum_q as (select username,
                             id,
                             "time",
                             ticker,
                             currency,
                             quantity * (- sign(payment))                                             as quantity,
                             price,
                             sum(quantity * (- sign(payment))) over (partition by ticker order by id) as quantity_cum
                      from trading_operations
                      where instrument_type = 'Stock'
                        and (type in ('Buy', 'Sell'))),
     events_id as (select x.username,
                          x.id,
                          x."time",
                          x.ticker,
                          x.currency,
                          x.quantity,
                          x.price,
                          x.quantity_cum,
                          (select y.id
                           from events_cum_q y
                           where y.quantity_cum > 0
                             and y.ticker = x.ticker
                             and y.id <= x.id
                             and y.quantity_cum = y.quantity
                           order by y.id desc
                           limit 1) as buy_id
                   from events_cum_q x),
     events as (select t.username,
                       t.ticker,
                       t.currency,
                       t.rn,
                       t.buy_time,
                       t.buy_price,
                       t.sell_time,
                       t.sell_price
                from ((select username,
                              id,
                              "time",
                              ticker,
                              currency,
                              quantity,
                              price,
                              quantity_cum,
                              buy_id,
                              generate_series,
                              price                                               as buy_price,
                              "time"                                              as buy_time,
                              row_number() over (partition by ticker order by id) as rn
                       from events_id,
                            lateral generate_series(1, abs(quantity)) generate_series(generate_series)
                       where quantity > 0) x
                    full join (select ticker,
                                      price                                               as sell_price,
                                      "time"                                              as sell_time,
                                      row_number() over (partition by ticker order by id) as rn
                               from events_id,
                                    lateral generate_series(1, abs(quantity)) generate_series(generate_series)
                               where quantity < 0) y using (rn, ticker)) t)
SELECT events.ticker,
       events.currency,
       events.buy_time,
       events.buy_price,
       t.quantity,
       events.sell_time,
       events.sell_price,
       events.username
from events
         join (select ticker,
                      max(rn)  as rn,
                      count(1) as quantity
               from events
               group by ticker, buy_time, sell_time) t using (rn, ticker)
order by (sell_time is not null), ticker, buy_time, sell_time;