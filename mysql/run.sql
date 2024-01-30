show databases;

use Online_Store;

show tables;

select * from Customers;

select * from Orders;

show create table Orders;

show create table Customers;


INSERT INTO Customers (FirstName, LastName, Email, Address) 
VALUES ('Alice', 'Brown', 'alice.brown@example.com', '1122 Birch Road, Lakeside');

INSERT INTO Customers (FirstName, LastName, Email, Address) 
VALUES ('Michael', 'Taylor', 'michael.taylor@example.com', '3344 Maple Drive, Hilltown');

INSERT INTO Customers (FirstName, LastName, Email, Address) 
VALUES ('Sara', 'Wilson', 'sara.wilson@example.com', '5566 Cedar Lane, Rivertown');


alter table Orders
modify OrderDate date;



select o.OrderID, o.OrderDate, c.CustomerID, c.FirstName, c.LastName
from Orders o inner join Customers c
on o.CustomerID = c.CustomerID;

show tables;

drop view product_info;


select count(*)  from Orders;

CREATE PROCEDURE `get_order_count` ()
BEGIN
    select count(*) from Orders;
END;


call get_order_count();

show PROCEDURE status;



select * from Customers
where firstName like 'a%';

SELECT 8 from customers
where firstName like '^a';





