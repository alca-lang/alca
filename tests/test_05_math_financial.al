// Copyright 2026 Alca Programming Language
// Everyday Test 05: Financial calculations, taxation, discounts, and rounding.

use std.math;

// 1. VAT / Sales Tax Calculation
func calculate_tax(net_amount: float, tax_rate: float) -> float:
    return net_amount * (tax_rate / 100.0);

// 2. Discount with coupon code
func apply_discount(subtotal: float, discount_percent: float) -> float:
    discount := subtotal * (discount_percent / 100.0);
    return subtotal - discount;

// 3. Round to 2 decimal places helper
func round_cents(amount: float) -> float:
    return float(math.round(amount * 100.0)) / 100.0;

// Test Order Calculation
item_price: float = 49.99;
quantity: int = 3;
subtotal := item_price * float(quantity); // 149.97

// Apply 10% holiday discount
discounted := apply_discount(subtotal, 10.0); // 134.973

// Add 20% VAT
tax := calculate_tax(discounted, 20.0); // 26.9946
total := round_cents(discounted + tax); // 161.97

assert total == 161.97;

// 4. Monthly loan payment interest approximation
principal: float = 10000.0;
annual_rate: float = 0.05; // 5%
monthly_rate: float = annual_rate / 12.0;
months: int = 12;

interest_accrued: float = principal * annual_rate;
assert interest_accrued == 500.0;

println("✓ test_05_math_financial passed");
