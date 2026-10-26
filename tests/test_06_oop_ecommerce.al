// Copyright 2026 Alca Programming Language
// Everyday Test 06: Domain modeling, structs, methods, order status workflow.

struct Customer:
    id: int;
    name: string;
    tier: string = "standard"; // standard, silver, gold

struct OrderItem:
    sku: string;
    title: string;
    unit_price: float;
    quantity: int;

struct Order:
    id: int;
    customer_id: int;
    items: list[OrderItem];
    status: string = "pending"; // pending, paid, shipped, delivered

func calculate_order_subtotal(order: Order) -> float:
    subtotal: float = 0.0;
    for item in order.items:
        subtotal = subtotal + (item.unit_price * float(item.quantity));
    return subtotal;

func apply_tier_discount(customer: Customer, amount: float) -> float:
    if customer.tier == "gold":
        return amount * 0.85; // 15% discount
    if customer.tier == "silver":
        return amount * 0.95; // 5% discount
    return amount;

// 1. Create Customer
vip_customer := Customer(101, "Елена Васильева", "gold");
assert vip_customer.tier == "gold";

// 2. Create Order Items
item1 := OrderItem("LAP-01", "Alca Pro Laptop", 1200.0, 1);
item2 := OrderItem("MOU-05", "Wireless Mouse", 50.0, 2);

// 3. Assemble Order
order := Order(5001, vip_customer.id, [item1, item2], "pending");
assert len(order.items) == 2;

raw_total := calculate_order_subtotal(order); // 1200 + 100 = 1300.0
assert raw_total == 1300.0;

final_total := apply_tier_discount(vip_customer, raw_total); // 1300 * 0.85 = 1105.0
assert final_total == 1105.0;

// 4. Status Transition
order.status = "paid";
assert order.status == "paid";

println("✓ test_06_oop_ecommerce passed");
