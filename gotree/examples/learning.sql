create database learning_order default character set utf8mb4 collate utf8mb4_unicode_ci;
create database learning_user default character set utf8mb4 collate utf8mb4_unicode_ci;
create database learning_product default character set utf8mb4 collate utf8mb4_unicode_ci;

USE `learning_order`;
CREATE TABLE `order` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `product_id` int(11) NOT NULL,
  `user_id` int(11) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `order_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `order_id` int(11) DEFAULT NULL,
  `desc` varchar(32) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

USE `learning_product`;
CREATE TABLE `product` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `price` int(11) NOT NULL,
  `desc` varchar(32) CHARACTER SET utf8 DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

LOCK TABLES `product` WRITE;
INSERT INTO `product` (`id`, `price`, `desc`)
VALUES
	(1,200,'iPhone');
UNLOCK TABLES;


USE `learning_user`;
CREATE TABLE `user` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(32) CHARACTER SET utf8 DEFAULT NULL,
  `money` int(11) DEFAULT NULL,
  `camelCase` varchar(32) NOT NULL DEFAULT '',
  `under_score_case` varchar(32) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

LOCK TABLES `user` WRITE;
INSERT INTO `user` (`id`, `name`, `money`, `camelCase`, `under_score_case`)
VALUES
	(1,'gotree',100400,'camelCase','under_score_case');
UNLOCK TABLES;
